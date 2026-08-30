package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/agentloop"
	"github.com/MajestaNet/ide/internal/authz"
)

// handleListClientPlaybooks is the runtime AgentSpec catalog. Definitions and
// instructions remain Metadata-owned; Client callers only receive fields needed
// to choose and start an active agent run.
func (s *Server) handleListClientPlaybooks(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT api_name, label, goal_template, require_approval,
       COALESCE(primary_section, ''), COALESCE(harness_id, ''), COALESCE(job_class, '')
FROM agent_playbooks
WHERE active=true
ORDER BY label, api_name`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var apiName, label, goalTemplate, primarySection, harnessID, jobClass string
		var requireApproval bool
		if err := rows.Scan(&apiName, &label, &goalTemplate, &requireApproval, &primarySection, &harnessID, &jobClass); err != nil {
			writeAPIError(w, err)
			return
		}
		list = append(list, map[string]any{
			"apiName": apiName, "label": label, "goalTemplate": goalTemplate,
			"requireApproval": requireApproval, "active": true,
			"primarySection": primarySection, "jobClass": jobClass, "harnessId": harnessID,
		})
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playbooks": list})
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}
	cursor := r.URL.Query().Get("cursor")
	// Over-fetch then filter for non-admin visibility (avoids leaking foreign record IDs).
	fetchLimit := limit
	if actor == nil || !actor.IsAdmin {
		fetchLimit = limit * 4
		if fetchLimit > 800 {
			fetchLimit = 800
		}
	}
	query := `
SELECT e.id::text, e.event_type, e.object_api_name, e.record_id::text, e.payload,
       e.created_at, e.published_at, e.attempts, e.last_error,
       r.owner_id::text, r.created_by_id::text
FROM outbox_events e
LEFT JOIN records r ON r.id = e.record_id AND r.object_api_name = e.object_api_name`
	args := []any{}
	if cursor != "" {
		args = append(args, cursor)
		query += ` WHERE e.created_at < $1::timestamptz`
	}
	query += ` ORDER BY e.created_at DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, fetchLimit)

	rs, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rs.Close()

	viewAll := map[string]struct{}{}
	if s.objectAz != nil && actor != nil && !actor.IsAdmin {
		viewAll, err = s.objectAz.GetViewAllObjects(r.Context(), actor)
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}

	events := []map[string]any{}
	var nextCursor string
	for rs.Next() {
		var id, eventType string
		var obj, recordID, lastError, ownerID, createdByID *string
		var payload []byte
		var created time.Time
		var published *time.Time
		var attempts int
		if err := rs.Scan(&id, &eventType, &obj, &recordID, &payload, &created, &published, &attempts, &lastError, &ownerID, &createdByID); err != nil {
			writeAPIError(w, err)
			return
		}
		var p any
		_ = json.Unmarshal(payload, &p)
		if !eventVisibleToActor(actor, obj, ownerID, createdByID, p, viewAll) {
			continue
		}
		p = redactEventPayloadFLS(r.Context(), s.fieldAz, actor, obj, p)
		m := map[string]any{
			"id": id, "eventType": eventType, "payload": p, "createdAt": created, "attempts": attempts,
		}
		if obj != nil {
			m["objectApiName"] = *obj
		}
		if recordID != nil {
			m["recordId"] = *recordID
		}
		if published != nil {
			m["publishedAt"] = *published
		}
		if lastError != nil {
			m["lastError"] = *lastError
		}
		events = append(events, m)
		if nextCursor == "" {
			nextCursor = created.UTC().Format(time.RFC3339Nano)
		}
		if len(events) >= limit {
			break
		}
	}
	out := map[string]any{"events": events}
	if len(events) == limit && nextCursor != "" {
		out["nextCursor"] = nextCursor
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListUnpublishedEvents(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	rs, err := pool.Query(r.Context(), `
SELECT e.id::text, e.event_type, e.object_api_name, e.record_id::text, e.payload,
       e.created_at, e.attempts, e.last_error,
       r.owner_id::text, r.created_by_id::text
FROM outbox_events e
LEFT JOIN records r ON r.id = e.record_id AND r.object_api_name = e.object_api_name
WHERE e.published_at IS NULL
ORDER BY e.created_at ASC
LIMIT 100`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rs.Close()

	viewAll := map[string]struct{}{}
	if s.objectAz != nil && actor != nil && !actor.IsAdmin {
		viewAll, err = s.objectAz.GetViewAllObjects(r.Context(), actor)
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}

	events := []map[string]any{}
	for rs.Next() {
		var id, eventType string
		var obj, recordID, lastError, ownerID, createdByID *string
		var payload []byte
		var created time.Time
		var attempts int
		if err := rs.Scan(&id, &eventType, &obj, &recordID, &payload, &created, &attempts, &lastError, &ownerID, &createdByID); err != nil {
			writeAPIError(w, err)
			return
		}
		var p any
		_ = json.Unmarshal(payload, &p)
		if !eventVisibleToActor(actor, obj, ownerID, createdByID, p, viewAll) {
			continue
		}
		p = redactEventPayloadFLS(r.Context(), s.fieldAz, actor, obj, p)
		m := map[string]any{"id": id, "eventType": eventType, "payload": p, "createdAt": created, "attempts": attempts}
		if obj != nil {
			m["objectApiName"] = *obj
		}
		if recordID != nil {
			m["recordId"] = *recordID
		}
		if lastError != nil {
			m["lastError"] = *lastError
		}
		events = append(events, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleAckEvent(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	id := r.PathValue("id")

	var eventType string
	var obj, recordID *string
	var payload []byte
	var ownerID, createdByID *string
	err := pool.QueryRow(r.Context(), `
SELECT e.event_type, e.object_api_name, e.record_id::text, e.payload,
       r.owner_id::text, r.created_by_id::text
FROM outbox_events e
LEFT JOIN records r ON r.id = e.record_id AND r.object_api_name = e.object_api_name
WHERE e.id=$1::uuid`, id).Scan(&eventType, &obj, &recordID, &payload, &ownerID, &createdByID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Event not found")
		return
	}
	var p any
	_ = json.Unmarshal(payload, &p)
	viewAll := map[string]struct{}{}
	if s.objectAz != nil && actor != nil && !actor.IsAdmin {
		viewAll, err = s.objectAz.GetViewAllObjects(r.Context(), actor)
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}
	if !eventVisibleToActor(actor, obj, ownerID, createdByID, p, viewAll) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Not allowed")
		return
	}

	var created, published time.Time
	err = pool.QueryRow(r.Context(), `
UPDATE outbox_events SET published_at=now() WHERE id=$1::uuid
RETURNING event_type, created_at, published_at`, id).Scan(&eventType, &created, &published)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Event not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "eventType": eventType, "createdAt": created, "publishedAt": published})
}

// eventVisibleToActor scopes outbox rows to admin, viewAll, record owner/creator, or the acting principal.
func eventVisibleToActor(
	actor *authz.Actor,
	objectAPIName, ownerID, createdByID *string,
	payload any,
	viewAll map[string]struct{},
) bool {
	if actor == nil {
		return false
	}
	if actor.IsAdmin {
		return true
	}
	obj := ""
	if objectAPIName != nil {
		obj = *objectAPIName
	}
	owner := ""
	if ownerID != nil {
		owner = *ownerID
	}
	createdBy := ""
	if createdByID != nil {
		createdBy = *createdByID
	}
	if obj != "" && authz.CanViewRecord(actor, owner, createdBy, obj, viewAll) {
		return true
	}
	// Events without a visible record: only the actor who produced them.
	if m, ok := payload.(map[string]any); ok {
		if aid, _ := m["actorId"].(string); aid != "" && aid == actor.ID {
			return true
		}
	}
	return false
}

// redactEventPayload strips record field dumps from low-privilege clients when FLS is unavailable.
func redactEventPayload(actor *authz.Actor, payload any) any {
	return redactEventPayloadFLS(context.Background(), nil, actor, nil, payload)
}

// redactEventPayloadFLS prefers field-level strip of data/patch maps; falls back to full redact.
func redactEventPayloadFLS(ctx context.Context, fieldAz *authz.FieldAuthz, actor *authz.Actor, objectAPIName *string, payload any) any {
	if actor != nil && actor.IsAdmin {
		return payload
	}
	m, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	out := make(map[string]any, len(m))
	obj := ""
	if objectAPIName != nil {
		obj = *objectAPIName
	}
	for k, v := range m {
		switch k {
		case "data", "patch":
			if fieldAz != nil && obj != "" {
				if dm, ok := v.(map[string]any); ok {
					// Copy before mutate.
					cp := make(map[string]any, len(dm))
					for fk, fv := range dm {
						cp[fk] = fv
					}
					stripped, err := fieldAz.StripUnreadableFields(ctx, actor, obj, cp)
					if err == nil {
						out[k] = stripped
						continue
					}
				}
			}
			continue
		default:
			out[k] = v
		}
	}
	return out
}

var templateVar = regexp.MustCompile(`\{\{(\w+)\}\}`)

func (s *Server) handleCreateAgentRun(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	var body struct {
		Goal            *string        `json:"goal"`
		PlaybookAPIName *string        `json:"playbookApiName"`
		Input           map[string]any `json:"input"`
		DryRun          bool           `json:"dryRun"`
		Approved        bool           `json:"approved"`
		Stream          bool           `json:"stream"`
		ConversationID  *string        `json:"conversationId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.Input == nil {
		body.Input = map[string]any{}
	}

	var playbookGoal *string
	var requireApproval bool
	var allowedTools, objectScopes, allowedSkills []byte
	var instructions string
	var primarySection, harnessID, harnessVersion, jobClass string
	if body.PlaybookAPIName != nil {
		var goal string
		err := pool.QueryRow(r.Context(), `
SELECT goal_template, require_approval, allowed_tools, object_scopes,
       COALESCE(allowed_skills, '[]'::jsonb), COALESCE(instructions, ''),
       COALESCE(primary_section, ''), COALESCE(harness_id, ''), COALESCE(harness_version, ''),
       COALESCE(job_class, '')
FROM agent_playbooks WHERE api_name=$1 AND active=true`,
			*body.PlaybookAPIName).Scan(&goal, &requireApproval, &allowedTools, &objectScopes, &allowedSkills, &instructions,
			&primarySection, &harnessID, &harnessVersion, &jobClass)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeErr(w, http.StatusNotFound, "NOT_FOUND", "active AgentSpec not found")
			} else {
				writeAPIError(w, err)
			}
			return
		}
		playbookGoal = &goal
	}
	if instructions != "" {
		if _, exists := body.Input["instructions"]; !exists {
			body.Input["instructions"] = instructions
		}
	}
	customerInstructions := instructions
	if v, ok := body.Input["instructions"].(string); ok && v != "" {
		customerInstructions = v
	}
	tools := []string{}
	if len(allowedTools) > 0 {
		if err := json.Unmarshal(allowedTools, &tools); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	scopes := []string{}
	if len(objectScopes) > 0 {
		if err := json.Unmarshal(objectScopes, &scopes); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	skills := []string{}
	if len(allowedSkills) > 0 {
		if err := json.Unmarshal(allowedSkills, &skills); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	applied := agentharness.Apply(agentharness.Spec{
		PrimarySection:  primarySection,
		JobClass:        jobClass,
		HarnessID:       harnessID,
		HarnessVersion:  harnessVersion,
		Instructions:    customerInstructions,
		AllowedTools:    tools,
		RequireApproval: requireApproval,
	})
	requireApproval = applied.RequireApproval
	tools = applied.AllowedTools

	goal := ""
	if body.Goal != nil {
		goal = *body.Goal
	} else if playbookGoal != nil {
		goal = *playbookGoal
		goal = templateVar.ReplaceAllStringFunc(goal, func(m string) string {
			key := templateVar.FindStringSubmatch(m)[1]
			if v, ok := body.Input[key]; ok {
				return stringify(v)
			}
			return m
		})
	}
	if goal == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "goal is required")
		return
	}

	stream := wantsAgentStream(r, body.Stream)
	inputJSON, _ := json.Marshal(body.Input)
	status := "queued"
	// Streaming create generates immediately; approval gates mutations/tools, not the LLM reply.
	if requireApproval && !body.Approved && !stream {
		status = "awaiting_approval"
	}
	var actorID *string
	if actor != nil {
		actorID = &actor.ID
	}
	var runID string
	var created time.Time
	err := pool.QueryRow(r.Context(), `
INSERT INTO agent_runs (playbook_api_name, status, goal, input, actor_id, dry_run, conversation_id)
VALUES ($1,$2,$3,$4::jsonb,$5::uuid,$6,$7::uuid)
RETURNING id::text, created_at`,
		body.PlaybookAPIName, status, goal, string(inputJSON), actorID, body.DryRun, body.ConversationID,
	).Scan(&runID, &created)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	resp := map[string]any{
		"id": runID, "playbookApiName": body.PlaybookAPIName, "status": status,
		"goal": goal, "input": body.Input, "actorId": actorID, "dryRun": body.DryRun, "createdAt": created,
		"harness": applied.Meta(),
	}

	if status == "awaiting_approval" {
		writeJSON(w, http.StatusAccepted, resp)
		return
	}

	if stream {
		playbook := ""
		if body.PlaybookAPIName != nil {
			playbook = *body.PlaybookAPIName
		}
		s.streamAgentRunLLM(w, r, agentLoopHTTPInput{
			runID: runID, goal: goal, playbook: playbook,
			input: body.Input, dryRun: body.DryRun, applied: applied,
			skills: skills, scopes: scopes,
		})
		return
	}

	if err := s.enqueueAgentRunJob(r.Context(), map[string]any{
		"runId": runID, "goal": goal, "dryRun": body.DryRun,
		"playbookApiName": body.PlaybookAPIName, "allowedTools": tools,
		"objectScopes": scopes, "allowedSkills": skills, "input": body.Input,
		"primarySection": applied.PrimarySection, "jobClass": applied.JobClass,
		"harnessId": applied.HarnessID, "harnessVersion": applied.HarnessVersion,
	}); err != nil {
		_, _ = pool.Exec(r.Context(), `UPDATE agent_runs SET status='failed', error=$2, completed_at=now() WHERE id=$1::uuid`, runID, err.Error())
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) enqueueAgentRunJob(ctx context.Context, payload map[string]any) error {
	if s.pool == nil {
		return fmt.Errorf("database unavailable")
	}
	jobPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO jobs (job_type, payload) VALUES ('agent.run', $1::jsonb)`, string(jobPayload))
	return err
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func (s *Server) handleGetAgentRun(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var id, status, goal string
	var playbook, actorID, errMsg *string
	var input, output []byte
	var dryRun bool
	var created time.Time
	var completed *time.Time
	err := pool.QueryRow(r.Context(), `
SELECT id::text, playbook_api_name, status, goal, input, output, actor_id::text, dry_run, error, created_at, completed_at
FROM agent_runs WHERE id=$1::uuid`, r.PathValue("id")).Scan(
		&id, &playbook, &status, &goal, &input, &output, &actorID, &dryRun, &errMsg, &created, &completed)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Agent run not found")
		return
	}
	if !s.canReadAgentRun(r, actorID) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Agent run not found")
		return
	}
	var in, out any
	_ = json.Unmarshal(input, &in)
	_ = json.Unmarshal(output, &out)
	m := map[string]any{"id": id, "status": status, "goal": goal, "input": in, "output": out, "dryRun": dryRun, "createdAt": created}
	if playbook != nil {
		m["playbookApiName"] = *playbook
	}
	if actorID != nil {
		m["actorId"] = *actorID
	}
	if errMsg != nil {
		m["error"] = *errMsg
	}
	if completed != nil {
		m["completedAt"] = *completed
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) canReadAgentRun(r *http.Request, runActorID *string) bool {
	actor := ActorFromContext(r.Context())
	if actor == nil {
		return false
	}
	if runActorID != nil && *runActorID == actor.ID {
		return true
	}
	if authz.HasAdminPrivilege(actor) {
		return true
	}
	return s.systemAz != nil && s.systemAz.AssertCapability(r.Context(), actor, authz.CapGovernAgents) == nil
}

func (s *Server) handleApproveAgentRun(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	id := r.PathValue("id")
	var approveBody struct {
		Stream bool `json:"stream"`
	}
	_ = json.NewDecoder(r.Body).Decode(&approveBody)
	stream := wantsAgentStream(r, approveBody.Stream)

	var currentStatus, goal string
	var dryRun bool
	var playbook *string
	var input []byte
	err := pool.QueryRow(r.Context(), `
SELECT status, goal, dry_run, playbook_api_name, input
FROM agent_runs WHERE id=$1::uuid`, id).Scan(&currentStatus, &goal, &dryRun, &playbook, &input)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Agent run not found")
		return
	}
	resume := currentStatus == agentloop.StatusAwaitingToolApproval
	if currentStatus != "awaiting_approval" && !resume {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Run not awaiting approval")
		return
	}
	tag, err := pool.Exec(r.Context(), `
UPDATE agent_runs SET status='queued' WHERE id=$1::uuid AND status=$2`, id, currentStatus)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Run not awaiting approval")
		return
	}
	var in map[string]any
	_ = json.Unmarshal(input, &in)

	tools := []string{}
	scopes := []string{}
	skills := []string{}
	var primarySection, harnessID, harnessVersion, instructions, jobClass string
	var requireApproval bool
	playbookName := ""
	if playbook != nil && *playbook != "" {
		playbookName = *playbook
		var allowedTools, objectScopes, allowedSkills []byte
		err := pool.QueryRow(r.Context(), `
SELECT allowed_tools, object_scopes, COALESCE(allowed_skills, '[]'::jsonb),
       COALESCE(instructions, ''), require_approval,
       COALESCE(primary_section, ''), COALESCE(harness_id, ''), COALESCE(harness_version, ''),
       COALESCE(job_class, '')
FROM agent_playbooks WHERE api_name=$1 AND active=true`,
			*playbook).Scan(&allowedTools, &objectScopes, &allowedSkills, &instructions, &requireApproval,
			&primarySection, &harnessID, &harnessVersion, &jobClass)
		if err == nil {
			for _, item := range []struct {
				raw  []byte
				dest *[]string
			}{
				{allowedTools, &tools}, {objectScopes, &scopes}, {allowedSkills, &skills},
			} {
				if err := json.Unmarshal(item.raw, item.dest); err != nil {
					_, _ = pool.Exec(r.Context(), `UPDATE agent_runs SET status=$2 WHERE id=$1::uuid AND status='queued'`, id, currentStatus)
					writeAPIError(w, err)
					return
				}
			}
		} else {
			_, _ = pool.Exec(r.Context(), `UPDATE agent_runs SET status=$2 WHERE id=$1::uuid AND status='queued'`, id, currentStatus)
			if errors.Is(err, pgx.ErrNoRows) {
				writeErr(w, http.StatusNotFound, "NOT_FOUND", "active AgentSpec not found")
			} else {
				writeAPIError(w, err)
			}
			return
		}
	}
	customerInstructions := instructions
	if v, ok := in["instructions"].(string); ok && v != "" {
		customerInstructions = v
	}
	applied := agentharness.Apply(agentharness.Spec{
		PrimarySection:  primarySection,
		JobClass:        jobClass,
		HarnessID:       harnessID,
		HarnessVersion:  harnessVersion,
		Instructions:    customerInstructions,
		AllowedTools:    tools,
		RequireApproval: requireApproval,
	})
	tools = applied.AllowedTools

	if stream {
		s.streamAgentRunLLM(w, r, agentLoopHTTPInput{
			runID: id, goal: goal, playbook: playbookName,
			input: in, dryRun: dryRun, resume: resume, applied: applied,
			skills: skills, scopes: scopes,
		})
		return
	}

	payload := map[string]any{
		"runId": id, "goal": goal, "dryRun": dryRun, "playbookApiName": playbook,
		"allowedTools": tools, "objectScopes": scopes, "allowedSkills": skills, "input": in,
		"primarySection": applied.PrimarySection, "jobClass": applied.JobClass,
		"harnessId": applied.HarnessID, "harnessVersion": applied.HarnessVersion,
	}
	if resume {
		payload["resume"] = true
	}
	if err := s.enqueueAgentRunJob(r.Context(), payload); err != nil {
		_, _ = pool.Exec(r.Context(), `UPDATE agent_runs SET status=$2 WHERE id=$1::uuid AND status='queued'`, id, currentStatus)
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "status": "queued", "goal": goal, "dryRun": dryRun, "harness": applied.Meta(), "resume": resume,
	})
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	rs, err := pool.Query(r.Context(), `
SELECT id::text, actor_id::text, action, object_api_name, record_id::text, details, created_at
FROM audit_log ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rs.Close()
	list := []map[string]any{}
	for rs.Next() {
		var id, action string
		var actorID, obj, recordID *string
		var details []byte
		var created time.Time
		if err := rs.Scan(&id, &actorID, &action, &obj, &recordID, &details, &created); err != nil {
			writeAPIError(w, err)
			return
		}
		var d any
		_ = json.Unmarshal(details, &d)
		m := map[string]any{"id": id, "action": action, "details": d, "createdAt": created}
		if actorID != nil {
			m["actorId"] = *actorID
		}
		if obj != nil {
			m["objectApiName"] = *obj
		}
		if recordID != nil {
			m["recordId"] = *recordID
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": list})
}
