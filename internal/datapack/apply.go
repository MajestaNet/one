package datapack

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// OrgClient is a minimal Client-family HTTP client (Bearer JWT or API key).
type OrgClient struct {
	BaseURL        string
	Bearer         string
	HTTP           *http.Client
	ApiRevisionPin int // One-API-Revision when > 0 (ADR-025).
}

func (c *OrgClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *OrgClient) doJSON(method, path string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			return nil, 0, err
		}
		body = &buf
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Bearer)
	if c.ApiRevisionPin > 0 {
		req.Header.Set("One-API-Revision", strconv.Itoa(c.ApiRevisionPin))
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	return b, resp.StatusCode, err
}

func (c *OrgClient) doRaw(method, path, contentType string, payload []byte) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Bearer)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	return b, resp.StatusCode, err
}

const ingestJobRowThreshold = 500

// ApplyOptions configures peer-sourced apply.
type ApplyOptions struct {
	RepoRoot string
	PackDir  string
	Source   *OrgClient // nil when --offline
	Target   *OrgClient
	Offline  bool
	MaxRows  int
	OnStep   func(step Step, pulled, upserted, failed int)
	// OnIngestWait is invoked while polling a target ingest job (tests drive the worker).
	OnIngestWait func()
	// IngestPollTimeout defaults to 2 minutes.
	IngestPollTimeout time.Duration
}

// ApplyReport summarizes apply outcomes.
type ApplyReport struct {
	Steps []StepReport `json:"steps"`
}

// StepReport is one step outcome.
type StepReport struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Pulled   int    `json:"pulled"`
	Upserted int    `json:"upserted"`
	Failed   int    `json:"failed"`
	Error    string `json:"error,omitempty"`
}

// Apply runs ordered steps: pull from source peer (or file) → upsert to target.
func Apply(m *Manifest, opts ApplyOptions) (*ApplyReport, error) {
	if opts.Target == nil {
		return nil, fmt.Errorf("target org client required")
	}
	if opts.MaxRows <= 0 {
		opts.MaxRows = 50_000
	}
	if errs := Validate(m, opts.PackDir, opts.RepoRoot); len(errs) > 0 {
		return nil, errs[0]
	}
	ordered, err := OrderSteps(m.Steps)
	if err != nil {
		return nil, err
	}
	if !opts.Offline && opts.Source == nil && strings.TrimSpace(m.SourceEnv) != "" {
		return nil, fmt.Errorf("sourceEnv %q set but source client is nil (pass --source-alias)", m.SourceEnv)
	}

	report := &ApplyReport{}
	// Cache parent external-id → target Id for reference rewrite within this apply.
	parentMap := map[string]map[string]string{} // object -> extId -> one Id

	for _, st := range ordered {
		sr := StepReport{ID: st.ID, Object: st.Object}
		rows, err := loadStepRows(m, st, opts)
		if err != nil {
			sr.Error = err.Error()
			report.Steps = append(report.Steps, sr)
			return report, err
		}
		sr.Pulled = len(rows)
		if len(rows) > opts.MaxRows {
			return report, fmt.Errorf("step %s: %d rows exceeds max %d", st.ID, len(rows), opts.MaxRows)
		}

		prepared := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if err := rewriteReferences(st, row, opts.Source, parentMap); err != nil {
				sr.Failed++
				continue
			}
			body := cloneRow(row)
			delete(body, "Id")
			for _, ref := range st.References {
				if ref.From != "" {
					delete(body, ref.From)
				}
			}
			prepared = append(prepared, body)
		}
		if len(rows) > ingestJobRowThreshold {
			upserted, failed, err := applyViaIngest(opts, st, prepared, parentMap)
			sr.Upserted += upserted
			sr.Failed += failed
			if err != nil {
				sr.Error = err.Error()
				if opts.OnStep != nil {
					opts.OnStep(st, sr.Pulled, sr.Upserted, sr.Failed)
				}
				report.Steps = append(report.Steps, sr)
				return report, err
			}
		} else {
			for _, body := range prepared {
				extField := st.ExternalIDField
				extVal := body[extField]
				status, err := upsertRow(opts.Target, st.Object, extField, extVal, body)
				if err != nil || status >= 300 {
					sr.Failed++
					continue
				}
				sr.Upserted++
				if id, ok := body["Id"].(string); ok && id != "" {
					ensureParentMap(parentMap, st.Object)[fmt.Sprint(extVal)] = id
				}
			}
		}
		if opts.OnStep != nil {
			opts.OnStep(st, sr.Pulled, sr.Upserted, sr.Failed)
		}
		report.Steps = append(report.Steps, sr)
	}
	return report, nil
}

func loadStepRows(m *Manifest, st Step, opts ApplyOptions) ([]map[string]any, error) {
	if opts.Offline || opts.Source == nil {
		if st.File == "" {
			return nil, fmt.Errorf("step %s: offline apply requires file", st.ID)
		}
		return readJSONL(filepath.Join(opts.PackDir, st.File))
	}
	// Prefer live query when present; else file.
	if st.Query != nil || st.File == "" {
		return querySource(opts.Source, st, opts.MaxRows)
	}
	return readJSONL(filepath.Join(opts.PackDir, st.File))
}

func querySource(src *OrgClient, st Step, maxRows int) ([]map[string]any, error) {
	selectFields := []string{"Id"}
	if st.Query != nil && len(st.Query.Select) > 0 {
		selectFields = st.Query.Select
	}
	if st.ExternalIDField != "" {
		found := false
		for _, f := range selectFields {
			if f == st.ExternalIDField {
				found = true
				break
			}
		}
		if !found {
			selectFields = append(selectFields, st.ExternalIDField)
		}
	}
	for _, ref := range st.References {
		found := false
		for _, f := range selectFields {
			if f == ref.Field {
				found = true
				break
			}
		}
		if !found {
			selectFields = append(selectFields, ref.Field)
		}
	}
	payload := map[string]any{
		"object": st.Object,
		"select": selectFields,
		"limit":  maxRows,
	}
	raw, status, err := src.doJSON(http.MethodPost, "/client/v1/query", payload)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("source query HTTP %d: %s", status, truncate(string(raw), 300))
	}
	var out struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode query: %w", err)
	}
	return out.Records, nil
}

func rewriteReferences(st Step, row map[string]any, source *OrgClient, parentMap map[string]map[string]string) error {
	for _, ref := range st.References {
		var sourceParentID string
		if ref.From != "" {
			if v, ok := row[ref.From]; ok && v != nil {
				// Offline: from column holds parent external id already.
				parentExt := fmt.Sprint(v)
				targetID, ok := ensureParentMap(parentMap, ref.ToObject)[parentExt]
				if !ok {
					return fmt.Errorf("missing parent %s.%s=%s in apply cache", ref.ToObject, ref.ToExternalIDField, parentExt)
				}
				row[ref.Field] = targetID
				continue
			}
		}
		if v, ok := row[ref.Field]; ok && v != nil {
			sourceParentID = fmt.Sprint(v)
		}
		if sourceParentID == "" {
			continue
		}
		parentExt, err := resolveSourceExternalID(source, ref.ToObject, ref.ToExternalIDField, sourceParentID, parentMap)
		if err != nil {
			return err
		}
		targetID, ok := ensureParentMap(parentMap, ref.ToObject)[parentExt]
		if !ok {
			// Parent should have been upserted in an earlier step; look up on target via later upsert cache miss.
			return fmt.Errorf("parent %s external id %s not applied yet", ref.ToObject, parentExt)
		}
		row[ref.Field] = targetID
	}
	return nil
}

func resolveSourceExternalID(source *OrgClient, object, extField, sourceID string, parentMap map[string]map[string]string) (string, error) {
	// If we already cached by Majesta One Id from a prior pull in this process, prefer map scan.
	for ext, id := range ensureParentMap(parentMap, object) {
		if id == sourceID {
			return ext, nil
		}
	}
	if source == nil {
		return "", fmt.Errorf("cannot resolve parent external id without source client")
	}
	raw, status, err := source.doJSON(http.MethodGet, "/client/v1/sobjects/"+object+"/"+sourceID, nil)
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("get parent %s/%s: HTTP %d", object, sourceID, status)
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		return "", err
	}
	ext := rec[extField]
	if ext == nil || fmt.Sprint(ext) == "" {
		return "", fmt.Errorf("parent %s/%s missing %s", object, sourceID, extField)
	}
	s := fmt.Sprint(ext)
	ensureParentMap(parentMap, object)[s] = "" // placeholder until target upsert fills Id
	return s, nil
}

func applyViaIngest(opts ApplyOptions, st Step, rows []map[string]any, parentMap map[string]map[string]string) (upserted, failed int, err error) {
	if len(rows) == 0 {
		return 0, 0, nil
	}
	target := opts.Target
	createBody := map[string]any{
		"object":          st.Object,
		"operation":       "upsert",
		"externalIdField": st.ExternalIDField,
		"contentType":     "application/x-ndjson",
		"allOrNone":       false,
	}
	raw, status, err := target.doJSON(http.MethodPost, "/client/v1/jobs/ingest", createBody)
	if err != nil {
		return 0, 0, err
	}
	if status >= 300 {
		return 0, 0, fmt.Errorf("create ingest job HTTP %d: %s", status, truncate(string(raw), 300))
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return 0, 0, fmt.Errorf("decode ingest job: %w", err)
	}
	var ndjson bytes.Buffer
	for _, row := range rows {
		b, merr := json.Marshal(row)
		if merr != nil {
			failed++
			continue
		}
		ndjson.Write(b)
		ndjson.WriteByte('\n')
	}
	raw, status, err = target.doRaw(http.MethodPut, "/client/v1/jobs/ingest/"+created.ID+"/batches", "application/x-ndjson", ndjson.Bytes())
	if err != nil {
		return 0, failed, err
	}
	if status >= 300 {
		return 0, failed, fmt.Errorf("ingest batch HTTP %d: %s", status, truncate(string(raw), 300))
	}
	raw, status, err = target.doJSON(http.MethodPatch, "/client/v1/jobs/ingest/"+created.ID, map[string]any{"state": "UploadComplete"})
	if err != nil {
		return 0, failed, err
	}
	if status >= 300 {
		return 0, failed, fmt.Errorf("close ingest job HTTP %d: %s", status, truncate(string(raw), 300))
	}
	timeout := opts.IngestPollTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if opts.OnIngestWait != nil {
			opts.OnIngestWait()
		}
		raw, status, err = target.doJSON(http.MethodGet, "/client/v1/jobs/ingest/"+created.ID, nil)
		if err != nil {
			return 0, failed, err
		}
		if status >= 300 {
			return 0, failed, fmt.Errorf("get ingest job HTTP %d: %s", status, truncate(string(raw), 300))
		}
		var stResp struct {
			State        string `json:"state"`
			SuccessCount int    `json:"successCount"`
			FailureCount int    `json:"failureCount"`
		}
		if err := json.Unmarshal(raw, &stResp); err != nil {
			return 0, failed, err
		}
		switch stResp.State {
		case "JobComplete":
			okIDs, mapErr := mapIngestSuccessIDs(target, created.ID, st, rows, parentMap)
			if mapErr != nil {
				return stResp.SuccessCount, stResp.FailureCount + failed, mapErr
			}
			return okIDs, stResp.FailureCount + failed, nil
		case "Failed", "Aborted":
			return stResp.SuccessCount, stResp.FailureCount + failed, fmt.Errorf("ingest job %s %s", created.ID, stResp.State)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return 0, failed, fmt.Errorf("ingest job %s timed out", created.ID)
}

func mapIngestSuccessIDs(target *OrgClient, jobID string, st Step, rows []map[string]any, parentMap map[string]map[string]string) (int, error) {
	raw, status, err := target.doRaw(http.MethodGet, "/client/v1/jobs/ingest/"+jobID+"/successfulResults", "", nil)
	if err != nil {
		return 0, err
	}
	if status >= 300 {
		return 0, fmt.Errorf("successfulResults HTTP %d: %s", status, truncate(string(raw), 300))
	}
	mapped := 0
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var res map[string]any
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			continue
		}
		id, _ := res["Id"].(string)
		if id == "" {
			continue
		}
		extVal := res["externalId"]
		if extVal == nil {
			if n, ok := res["line"].(float64); ok {
				idx := int(n) - 1
				if idx >= 0 && idx < len(rows) {
					extVal = rows[idx][st.ExternalIDField]
				}
			}
		}
		if extVal != nil && fmt.Sprint(extVal) != "" {
			ensureParentMap(parentMap, st.Object)[fmt.Sprint(extVal)] = id
		}
		mapped++
	}
	return mapped, sc.Err()
}

func upsertRow(target *OrgClient, object, extField string, extVal any, body map[string]any) (int, error) {
	payload := cloneRow(body)
	payload["externalIdField"] = extField
	payload["externalId"] = extVal
	raw, status, err := target.doJSON(http.MethodPost, "/client/v1/sobjects/"+object+"/upsert", payload)
	if err != nil {
		return 0, err
	}
	if status < 300 {
		var rec map[string]any
		if json.Unmarshal(raw, &rec) == nil {
			if id, ok := rec["Id"].(string); ok {
				body["Id"] = id
			}
		}
	}
	return status, nil
}

func ensureParentMap(m map[string]map[string]string, object string) map[string]string {
	if m[object] == nil {
		m[object] = map[string]string{}
	}
	return m[object]
}

func readJSONL(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var rows []map[string]any
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 10*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		rows = append(rows, row)
	}
	return rows, sc.Err()
}

func cloneRow(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
