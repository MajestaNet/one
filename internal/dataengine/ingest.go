package dataengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/jackc/pgx/v5"
)

const (
	IngestStateOpen           = "Open"
	IngestStateUploadComplete = "UploadComplete"
	IngestStateInProgress     = "InProgress"
	IngestStateJobComplete    = "JobComplete"
	IngestStateFailed         = "Failed"
	IngestStateAborted        = "Aborted"

	IngestOpInsert = "insert"
	IngestOpUpdate = "update"
	IngestOpUpsert = "upsert"
	IngestOpDelete = "delete"

	IngestMaxRows         = 100_000
	IngestMaxUploadBytes  = 100 << 20
	IngestChunkSize       = 500
	IngestMaxOpenPerActor = 5
	IngestMaxInProgress   = 2
)

// IngestJob is a Client Bulk ingest job row.
type IngestJob struct {
	ID              string     `json:"id"`
	ActorID         string     `json:"actorId"`
	ObjectAPIName   string     `json:"object"`
	Operation       string     `json:"operation"`
	ExternalIDField *string    `json:"externalIdField,omitempty"`
	ContentType     string     `json:"contentType"`
	State           string     `json:"state"`
	UploadBytes     int64      `json:"uploadBytes"`
	RowCount        int        `json:"rowCount"`
	SuccessCount    int        `json:"successCount"`
	FailureCount    int        `json:"failureCount"`
	AllOrNone       bool       `json:"allOrNone"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

// CreateIngestJobInput creates an Open ingest job.
type CreateIngestJobInput struct {
	ObjectAPIName   string
	Operation       string
	ExternalIDField string
	ContentType     string
	AllOrNone       bool
}

func normalizeIngestOp(op string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case IngestOpInsert, IngestOpUpdate, IngestOpUpsert, IngestOpDelete:
		return strings.ToLower(strings.TrimSpace(op)), nil
	default:
		return "", validationErrorf("operation must be insert|update|upsert|delete")
	}
}

// CreateIngestJob inserts an Open job for the actor.
func (s *Service) CreateIngestJob(ctx context.Context, actor *authz.Actor, in CreateIngestJobInput) (*IngestJob, error) {
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		return nil, fmt.Errorf("%w: no actor", authz.ErrForbidden)
	}
	op, err := normalizeIngestOp(in.Operation)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ObjectAPIName) == "" {
		return nil, validationErrorf("object is required")
	}
	obj, err := s.meta.GetObject(ctx, in.ObjectAPIName)
	if err != nil {
		return nil, err
	}
	if err := rejectKernelStorage(in.ObjectAPIName, obj.StorageMode); err != nil {
		return nil, err
	}
	ext := strings.TrimSpace(in.ExternalIDField)
	if op == IngestOpUpsert && ext == "" {
		return nil, validationErrorf("externalIdField is required for upsert")
	}
	if ext != "" {
		desc, err := s.meta.Describe(ctx, in.ObjectAPIName)
		if err != nil {
			return nil, err
		}
		if _, err := requireExternalIDField(desc.Fields, ext); err != nil {
			return nil, err
		}
	}
	ct := strings.TrimSpace(in.ContentType)
	if ct == "" {
		ct = "application/x-ndjson"
	}
	if ct != "application/x-ndjson" && ct != "application/jsonl" {
		return nil, validationErrorf("contentType must be application/x-ndjson (CSV deferred)")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "ingest-open:"+actor.ID); err != nil {
		return nil, err
	}
	var openCount int
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*) FROM ingest_jobs
WHERE actor_id=$1::uuid AND state IN ('Open','UploadComplete','InProgress')`, actor.ID).Scan(&openCount); err != nil {
		return nil, err
	}
	if openCount >= IngestMaxOpenPerActor {
		return nil, validationErrorf("too many open ingest jobs (max %d)", IngestMaxOpenPerActor)
	}

	var extArg *string
	if ext != "" {
		extArg = &ext
	}
	row := tx.QueryRow(ctx, `
INSERT INTO ingest_jobs (actor_id, object_api_name, operation, external_id_field, content_type, all_or_none, state)
VALUES ($1::uuid,$2,$3,$4,$5,$6,'Open')
RETURNING id::text, actor_id::text, object_api_name, operation, external_id_field, content_type, state,
  upload_bytes, row_count, success_count, failure_count, all_or_none, error_message, created_at, completed_at`,
		actor.ID, in.ObjectAPIName, op, extArg, ct, in.AllOrNone)
	job, err := scanIngestJob(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}

func scanIngestJob(row pgx.Row) (*IngestJob, error) {
	var j IngestJob
	var ext *string
	var errMsg *string
	var completed *time.Time
	if err := row.Scan(
		&j.ID, &j.ActorID, &j.ObjectAPIName, &j.Operation, &ext, &j.ContentType, &j.State,
		&j.UploadBytes, &j.RowCount, &j.SuccessCount, &j.FailureCount, &j.AllOrNone, &errMsg, &j.CreatedAt, &completed,
	); err != nil {
		return nil, err
	}
	j.ExternalIDField = ext
	j.ErrorMessage = errMsg
	j.CompletedAt = completed
	return &j, nil
}

// GetIngestJob loads a job by id.
func (s *Service) GetIngestJob(ctx context.Context, id string) (*IngestJob, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id::text, actor_id::text, object_api_name, operation, external_id_field, content_type, state,
  upload_bytes, row_count, success_count, failure_count, all_or_none, error_message, created_at, completed_at
FROM ingest_jobs WHERE id=$1::uuid`, id)
	j, err := scanIngestJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: ingest job not found", ErrNotFound)
		}
		return nil, err
	}
	return j, nil
}

// AppendIngestBatch appends NDJSON bytes to an Open job.
func (s *Service) AppendIngestBatch(ctx context.Context, id string, actorID string, data []byte) (*IngestJob, error) {
	j, err := s.GetIngestJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.ActorID != actorID {
		return nil, fmt.Errorf("%w: not job owner", authz.ErrForbidden)
	}
	if j.State != IngestStateOpen {
		return nil, validationErrorf("job is not Open")
	}
	if int64(len(data))+j.UploadBytes > IngestMaxUploadBytes {
		return nil, validationErrorf("upload exceeds %d bytes", IngestMaxUploadBytes)
	}
	lines, err := countNDJSONLines(data)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
UPDATE ingest_jobs
SET payload = payload || $2::bytea,
    upload_bytes = upload_bytes + $3,
    row_count = row_count + $4
WHERE id=$1::uuid AND actor_id=$5::uuid AND state='Open'
  AND upload_bytes + $3 <= $6
  AND row_count + $4 <= $7
RETURNING id::text, actor_id::text, object_api_name, operation, external_id_field, content_type, state,
  upload_bytes, row_count, success_count, failure_count, all_or_none, error_message, created_at, completed_at`,
		id, data, len(data), lines, actorID, IngestMaxUploadBytes, IngestMaxRows)
	updated, err := scanIngestJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, validationErrorf("job is not Open or upload limits would be exceeded")
	}
	return updated, err
}

func countNDJSONLines(data []byte) (int, error) {
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	// Allow large lines
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 10*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		n++
	}
	if err := sc.Err(); err != nil {
		return 0, validationErrorf("NDJSON line exceeds maximum size: %v", err)
	}
	return n, nil
}

// CloseIngestJob marks UploadComplete and enqueues worker processing.
func (s *Service) CloseIngestJob(ctx context.Context, id, actorID string) (*IngestJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row := tx.QueryRow(ctx, `
UPDATE ingest_jobs SET state='UploadComplete'
WHERE id=$1::uuid AND actor_id=$2::uuid AND state='Open' AND row_count <= $3
RETURNING id::text, actor_id::text, object_api_name, operation, external_id_field, content_type, state,
  upload_bytes, row_count, success_count, failure_count, all_or_none, error_message, created_at, completed_at`,
		id, actorID, IngestMaxRows)
	j, err := scanIngestJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, validationErrorf("job is not Open, not owned by actor, or exceeds row limit")
	}
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]any{"ingestJobId": id})
	if _, err := tx.Exec(ctx, `INSERT INTO jobs (job_type, payload) VALUES ('ingest.process', $1::jsonb)`, string(payload)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return j, nil
}

// AbortIngestJob aborts an Open or UploadComplete job.
func (s *Service) AbortIngestJob(ctx context.Context, id, actorID string) (*IngestJob, error) {
	row := s.pool.QueryRow(ctx, `
UPDATE ingest_jobs SET state='Aborted', completed_at=now()
WHERE id=$1::uuid AND actor_id=$2::uuid AND state IN ('Open','UploadComplete')
RETURNING id::text, actor_id::text, object_api_name, operation, external_id_field, content_type, state,
  upload_bytes, row_count, success_count, failure_count, all_or_none, error_message, created_at, completed_at`, id, actorID)
	j, err := scanIngestJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, validationErrorf("job cannot be aborted")
	}
	return j, err
}

// IngestJobResults returns successful or failed NDJSON result bytes.
func (s *Service) IngestJobResults(ctx context.Context, id, actorID string, failed bool) ([]byte, error) {
	j, err := s.GetIngestJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.ActorID != actorID {
		return nil, fmt.Errorf("%w: not job owner", authz.ErrForbidden)
	}
	col := "result_success"
	if failed {
		col = "result_failed"
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT `+col+` FROM ingest_jobs WHERE id=$1::uuid`, id).Scan(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// claimIngestForProcess enforces the install-wide InProgress cap (max 2).
// Extra jobs stay UploadComplete and are re-enqueued with a short delay.
func (s *Service) claimIngestForProcess(ctx context.Context, id string) (proceed bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "ingest-in-progress"); err != nil {
		return false, err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT state FROM ingest_jobs WHERE id=$1::uuid`, id).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("%w: ingest job not found", ErrNotFound)
	}
	if err != nil {
		return false, err
	}
	switch state {
	case IngestStateJobComplete, IngestStateFailed, IngestStateAborted:
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	case IngestStateInProgress:
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	case IngestStateUploadComplete:
		var n int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM ingest_jobs WHERE state='InProgress'`).Scan(&n); err != nil {
			return false, err
		}
		if n >= IngestMaxInProgress {
			payload, _ := json.Marshal(map[string]any{"ingestJobId": id})
			if _, err := tx.Exec(ctx, `
INSERT INTO jobs (job_type, payload, run_at)
VALUES ('ingest.process', $1::jsonb, now() + interval '2 seconds')`, string(payload)); err != nil {
				return false, err
			}
			if err := tx.Commit(ctx); err != nil {
				return false, err
			}
			return false, nil
		}
		tag, err := tx.Exec(ctx, `UPDATE ingest_jobs SET state='InProgress' WHERE id=$1::uuid AND state='UploadComplete'`, id)
		if err != nil {
			return false, err
		}
		if tag.RowsAffected() == 0 {
			if err := tx.Commit(ctx); err != nil {
				return false, err
			}
			return false, nil
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, fmt.Errorf("ingest job %s not ready (state=%s)", id, state)
	}
}

type ingestWorkItem struct {
	lineNo   int
	row      map[string]any
	parseErr error
}

// ProcessIngestJob runs insert/update/upsert/delete for a closed job (worker).
func (s *Service) ProcessIngestJob(ctx context.Context, id string, az *UpsertAuthz, resolveActor func(ctx context.Context, actorID string) (*authz.Actor, error)) error {
	proceed, err := s.claimIngestForProcess(ctx, id)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	var (
		actorID, object, operation string
		extField                   *string
		allOrNone                  bool
		payload                    []byte
		persistedSuccess           int
		persistedFailure           int
	)
	if err := s.pool.QueryRow(ctx, `
SELECT actor_id::text, object_api_name, operation, external_id_field, all_or_none, payload,
       success_count, failure_count
FROM ingest_jobs WHERE id=$1::uuid`, id).Scan(
		&actorID, &object, &operation, &extField, &allOrNone, &payload, &persistedSuccess, &persistedFailure,
	); err != nil {
		return err
	}
	actor, err := resolveActor(ctx, actorID)
	if err != nil {
		return s.failIngestJob(ctx, id, err.Error())
	}
	if az == nil || az.AssertObjectAccess == nil || az.CanModifyRecord == nil || az.GetModifyAllObjects == nil || az.AssertEditableFields == nil {
		return s.failIngestJob(ctx, id, "ingest authz is not fully configured")
	}
	triggerEvent := "create"
	switch operation {
	case IngestOpUpdate, IngestOpUpsert:
		triggerEvent = "update"
	case IngestOpDelete:
		triggerEvent = "delete"
	}
	if err := s.ensureRecordPartitionsForWrite(ctx, object, triggerEvent); err != nil {
		return s.failIngestJob(ctx, id, err.Error())
	}
	if operation == IngestOpUpsert {
		if err := s.ensureRecordPartitionsForWrite(ctx, object, "create"); err != nil {
			return s.failIngestJob(ctx, id, err.Error())
		}
	}

	var work []ingestWorkItem
	sc := bufio.NewScanner(bytes.NewReader(payload))
	scanBuf := make([]byte, 0, 64*1024)
	sc.Buffer(scanBuf, 10*1024*1024)
	lineNo := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		lineNo++
		item := ingestWorkItem{lineNo: lineNo}
		if err := json.Unmarshal([]byte(line), &item.row); err != nil {
			item.parseErr = err
		}
		work = append(work, item)
	}
	if err := sc.Err(); err != nil {
		return s.failIngestJob(ctx, id, err.Error())
	}
	persistedProcessed := persistedSuccess + persistedFailure
	if persistedProcessed < 0 || persistedProcessed > len(work) || (allOrNone && persistedProcessed != 0) {
		return s.failIngestJob(ctx, id, "ingest progress is inconsistent with the uploaded row count")
	}

	var successBuf, failBuf bytes.Buffer
	success, failure := persistedSuccess, persistedFailure
	applyItems := func(runCtx context.Context, items []ingestWorkItem) error {
		tx := txFrom(runCtx)
		for _, item := range items {
			useSP := tx != nil && !allOrNone
			sp := fmt.Sprintf("ing_line_%d", item.lineNo)
			if useSP {
				if _, err := tx.Exec(runCtx, "SAVEPOINT "+sp); err != nil {
					return err
				}
			}
			rollbackSP := func() {
				if useSP {
					_, _ = tx.Exec(runCtx, "ROLLBACK TO SAVEPOINT "+sp)
				}
			}
			if item.parseErr != nil {
				rollbackSP()
				failure++
				writeIngestFail(&failBuf, item.lineNo, "", item.parseErr.Error())
				if allOrNone {
					return item.parseErr
				}
				continue
			}
			res, rowErr := s.applyIngestRow(runCtx, object, operation, extField, item.row, actor, az)
			if rowErr != nil {
				rollbackSP()
				failure++
				extVal := rowValueString(item.row, extField)
				writeIngestFail(&failBuf, item.lineNo, extVal, rowErr.Error())
				if allOrNone {
					return rowErr
				}
				continue
			}
			if useSP {
				if _, err := tx.Exec(runCtx, "RELEASE SAVEPOINT "+sp); err != nil {
					return err
				}
			}
			if res == nil {
				res = map[string]any{}
			}
			res["line"] = item.lineNo
			success++
			writeIngestSuccess(&successBuf, res)
		}
		return nil
	}

	if allOrNone {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return s.failIngestJob(ctx, id, err.Error())
		}
		defer func() { _ = tx.Rollback(ctx) }()
		processCtx := withTx(ctx, tx)
		if err := applyItems(processCtx, work); err != nil {
			_ = tx.Rollback(ctx)
			return s.finishIngestJob(ctx, id, IngestStateFailed, 0, failure, nil, failBuf.Bytes(), err.Error())
		}
		if err := s.finishIngestJobWith(processCtx, id, IngestStateJobComplete, success, failure, successBuf.Bytes(), failBuf.Bytes(), ""); err != nil {
			_ = tx.Rollback(ctx)
			return s.failIngestJob(ctx, id, err.Error())
		}
		if err := tx.Commit(ctx); err != nil {
			return s.failIngestJob(ctx, id, err.Error())
		}
		return nil
	}

	// Non-atomic jobs commit mutations and their progress checkpoint together.
	// A worker retry therefore resumes after the last committed row instead of
	// replaying inserts, mutations, triggers, and outbox events.
	for i := persistedProcessed; i < len(work); i += IngestChunkSize {
		end := i + IngestChunkSize
		if end > len(work) {
			end = len(work)
		}
		chunk := work[i:end]
		expectedProcessed := success + failure
		successBuf.Reset()
		failBuf.Reset()
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return s.failIngestJob(ctx, id, err.Error())
		}
		processCtx := withTx(ctx, tx)
		if err := applyItems(processCtx, chunk); err != nil {
			_ = tx.Rollback(ctx)
			return s.failIngestJob(ctx, id, err.Error())
		}
		if err := s.checkpointIngestChunkWith(processCtx, id, expectedProcessed, success, failure, successBuf.Bytes(), failBuf.Bytes()); err != nil {
			_ = tx.Rollback(ctx)
			// Another invocation may have committed the same checkpoint. Leave the
			// ingest state resumable and let the durable job retry converge.
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return s.failIngestJob(ctx, id, err.Error())
		}
	}
	if err := s.completeCheckpointedIngest(ctx, id, len(work)); err != nil {
		return s.failIngestJob(ctx, id, err.Error())
	}
	return nil
}

func (s *Service) checkpointIngestChunkWith(ctx context.Context, id string, expectedProcessed, success, failure int, okBytes, failBytes []byte) error {
	if okBytes == nil {
		okBytes = []byte{}
	}
	if failBytes == nil {
		failBytes = []byte{}
	}
	tag, err := s.querier(ctx).Exec(ctx, `
UPDATE ingest_jobs
SET success_count=$3, failure_count=$4,
    result_success=result_success || $5::bytea,
    result_failed=result_failed || $6::bytea
WHERE id=$1::uuid AND state='InProgress' AND success_count + failure_count=$2`,
		id, expectedProcessed, success, failure, okBytes, failBytes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("ingest progress changed concurrently")
	}
	return nil
}

func (s *Service) completeCheckpointedIngest(ctx context.Context, id string, processed int) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE ingest_jobs
SET state='JobComplete', error_message=NULL, completed_at=now()
WHERE id=$1::uuid AND state='InProgress' AND success_count + failure_count=$2`, id, processed)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		// Completion is idempotent: a duplicate worker invocation that arrives
		// after the first commit must not turn a successful ingest into Failed.
		var state string
		var success, failure int
		if err := s.pool.QueryRow(ctx, `
SELECT state, success_count, failure_count FROM ingest_jobs WHERE id=$1::uuid`, id).Scan(&state, &success, &failure); err != nil {
			return err
		}
		if state == IngestStateJobComplete && success+failure == processed {
			return nil
		}
		return fmt.Errorf("ingest progress is incomplete")
	}
	return nil
}

func (s *Service) applyIngestRow(
	ctx context.Context,
	object, operation string,
	extField *string,
	row map[string]any,
	actor *authz.Actor,
	az *UpsertAuthz,
) (map[string]any, error) {
	switch operation {
	case IngestOpInsert:
		if err := az.AssertObjectAccess(ctx, actor, object, authz.ActionCreate); err != nil {
			return nil, err
		}
		if err := assertIngestOwnerWrite(ctx, az, actor, object, row); err != nil {
			return nil, err
		}
		if err := az.AssertEditableFields(ctx, actor, object, row); err != nil {
			return nil, err
		}
		rec, err := s.Create(ctx, object, row, actor)
		if err != nil {
			return nil, err
		}
		return map[string]any{"Id": rec["Id"], "created": true}, nil
	case IngestOpUpsert:
		field := ""
		if extField != nil {
			field = *extField
		}
		val, ok := row[field]
		if !ok {
			return nil, validationErrorf("missing external id field %s", field)
		}
		body := cloneMap(row)
		if err := assertIngestOwnerWrite(ctx, az, actor, object, body); err != nil {
			return nil, err
		}
		result, err := s.Upsert(ctx, object, field, val, body, actor, az)
		if err != nil {
			return nil, err
		}
		return map[string]any{"Id": result.Record["Id"], "created": result.Created, "externalId": val}, nil
	case IngestOpUpdate:
		if err := az.AssertObjectAccess(ctx, actor, object, authz.ActionUpdate); err != nil {
			return nil, err
		}
		if id, _ := row["Id"].(string); strings.TrimSpace(id) != "" {
			existing, err := s.Get(ctx, object, id)
			if err != nil {
				return nil, err
			}
			if err := assertIngestCanModify(ctx, az, actor, object, existing); err != nil {
				return nil, err
			}
			body := cloneMap(row)
			delete(body, "Id")
			if err := assertIngestOwnerWrite(ctx, az, actor, object, body); err != nil {
				return nil, err
			}
			if err := az.AssertEditableFields(ctx, actor, object, body); err != nil {
				return nil, err
			}
			rec, err := s.Update(ctx, object, id, body, actor)
			if err != nil {
				return nil, err
			}
			return map[string]any{"Id": rec["Id"], "created": false}, nil
		}
		if extField != nil && *extField != "" {
			val, ok := row[*extField]
			if !ok {
				return nil, validationErrorf("update requires Id or %s", *extField)
			}
			existing, err := s.GetByExternalID(ctx, object, *extField, fmt.Sprint(val))
			if err != nil {
				return nil, err
			}
			if err := assertIngestCanModify(ctx, az, actor, object, existing); err != nil {
				return nil, err
			}
			recID, _ := existing["Id"].(string)
			body := cloneMap(row)
			delete(body, "Id")
			if err := assertIngestOwnerWrite(ctx, az, actor, object, body); err != nil {
				return nil, err
			}
			if err := az.AssertEditableFields(ctx, actor, object, body); err != nil {
				return nil, err
			}
			rec, err := s.Update(ctx, object, recID, body, actor)
			if err != nil {
				return nil, err
			}
			return map[string]any{"Id": rec["Id"], "created": false}, nil
		}
		return nil, validationErrorf("update requires Id or externalIdField")
	case IngestOpDelete:
		if err := az.AssertObjectAccess(ctx, actor, object, authz.ActionDelete); err != nil {
			return nil, err
		}
		if id, _ := row["Id"].(string); strings.TrimSpace(id) != "" {
			existing, err := s.Get(ctx, object, id)
			if err != nil {
				return nil, err
			}
			if err := assertIngestCanModify(ctx, az, actor, object, existing); err != nil {
				return nil, err
			}
			if err := s.Delete(ctx, object, id, actor); err != nil {
				return nil, err
			}
			return map[string]any{"Id": id, "deleted": true}, nil
		}
		if extField != nil && *extField != "" {
			val, ok := row[*extField]
			if !ok {
				return nil, validationErrorf("delete requires Id or %s", *extField)
			}
			existing, err := s.GetByExternalID(ctx, object, *extField, fmt.Sprint(val))
			if err != nil {
				return nil, err
			}
			if err := assertIngestCanModify(ctx, az, actor, object, existing); err != nil {
				return nil, err
			}
			recordID, _ := existing["Id"].(string)
			if err := s.Delete(ctx, object, recordID, actor); err != nil {
				return nil, err
			}
			return map[string]any{"externalId": val, "deleted": true}, nil
		}
		return nil, validationErrorf("delete requires Id or externalIdField")
	default:
		return nil, validationErrorf("unknown operation %s", operation)
	}
}

func assertIngestCanModify(ctx context.Context, az *UpsertAuthz, actor *authz.Actor, object string, rec SObjectRecord) error {
	modifyAll, err := az.GetModifyAllObjects(ctx, actor)
	if err != nil {
		return err
	}
	id, _ := rec["Id"].(string)
	ownerID, _ := rec["OwnerId"].(string)
	createdByID, _ := rec["CreatedById"].(string)
	ok, err := az.CanModifyRecord(ctx, actor, id, ownerID, createdByID, object, modifyAll)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: not allowed to modify record", authz.ErrForbidden)
	}
	return nil
}

func assertIngestOwnerWrite(ctx context.Context, az *UpsertAuthz, actor *authz.Actor, object string, body map[string]any) error {
	v, ok := body["OwnerId"]
	if !ok {
		return nil
	}
	modifyAll, err := az.GetModifyAllObjects(ctx, actor)
	if err != nil {
		return err
	}
	var ownerID *string
	if value, ok := v.(string); ok && value != "" {
		ownerID = &value
	}
	return authz.AssertOwnerIDWritable(actor, object, ownerID, modifyAll)
}

func writeIngestSuccess(buf *bytes.Buffer, res map[string]any) {
	b, _ := json.Marshal(res)
	buf.Write(b)
	buf.WriteByte('\n')
}

func writeIngestFail(buf *bytes.Buffer, line int, externalID, msg string) {
	b, _ := json.Marshal(map[string]any{"line": line, "externalId": externalID, "error": msg})
	buf.Write(b)
	buf.WriteByte('\n')
}

func rowValueString(row map[string]any, extField *string) string {
	if extField == nil || *extField == "" {
		return ""
	}
	if v, ok := row[*extField]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func (s *Service) failIngestJob(ctx context.Context, id, msg string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE ingest_jobs SET state='Failed', error_message=$2, completed_at=now()
WHERE id=$1::uuid AND state='InProgress'`, id, msg)
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", msg)
}

func (s *Service) finishIngestJob(ctx context.Context, id, state string, success, failure int, okBytes, failBytes []byte, errMsg string) error {
	return s.finishIngestJobWith(ctx, id, state, success, failure, okBytes, failBytes, errMsg)
}

func (s *Service) finishIngestJobWith(ctx context.Context, id, state string, success, failure int, okBytes, failBytes []byte, errMsg string) error {
	var msg *string
	if errMsg != "" {
		msg = &errMsg
	}
	if okBytes == nil {
		okBytes = []byte{}
	}
	if failBytes == nil {
		failBytes = []byte{}
	}
	_, err := s.querier(ctx).Exec(ctx, `
UPDATE ingest_jobs
SET state=$2, success_count=$3, failure_count=$4, result_success=$5, result_failed=$6,
    error_message=$7, completed_at=now()
WHERE id=$1::uuid`, id, state, success, failure, okBytes, failBytes, msg)
	return err
}
