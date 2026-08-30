package agentloop

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/inference"
)

func parkRun(ctx context.Context, pool *db.Pool, runID string, output map[string]any, msgs []inference.Message, park pendingPark, emit Sink) (Result, error) {
	output["pendingToolCall"] = park.Call
	output["loopTranscript"] = msgs
	output["loopRound"] = park.Call.Round
	if len(park.Remaining) > 0 {
		output["remainingToolCalls"] = park.Remaining
	} else {
		delete(output, "remainingToolCalls")
	}
	payload := map[string]any{"pendingToolCall": park.Call}
	emit(EventApprovalRequired, payload)
	return persistRun(ctx, pool, runID, StatusAwaitingToolApproval, output, "", false, emit)
}

func completeRun(ctx context.Context, pool *db.Pool, runID string, status string, output map[string]any, emit Sink) (Result, error) {
	delete(output, "pendingToolCall")
	delete(output, "remainingToolCalls")
	delete(output, "loopTranscript")
	res, err := persistRun(ctx, pool, runID, status, output, "", true, emit)
	if err == nil {
		emit(EventDone, map[string]any{"id": runID, "status": status, "output": output})
	}
	return res, err
}

func failRun(ctx context.Context, pool *db.Pool, runID, msg string, output map[string]any) Result {
	if output == nil {
		output = map[string]any{}
	}
	_, _ = persistRun(ctx, pool, runID, StatusFailed, output, msg, true, nil)
	return Result{Status: StatusFailed, Output: output, Error: msg}
}

func persistRun(ctx context.Context, pool *db.Pool, runID, status string, output map[string]any, errMsg string, done bool, _ Sink) (Result, error) {
	outJSON, err := json.Marshal(output)
	if err != nil {
		outJSON = []byte("{}")
	}
	var execErr error
	if done {
		if errMsg != "" {
			_, execErr = pool.Exec(ctx, `
UPDATE agent_runs SET status=$2, output=$3::jsonb, error=$4, completed_at=$5
WHERE id=$1::uuid`, runID, status, string(outJSON), errMsg, time.Now())
		} else {
			_, execErr = pool.Exec(ctx, `
UPDATE agent_runs SET status=$2, output=$3::jsonb, error=NULL, completed_at=$4
WHERE id=$1::uuid`, runID, status, string(outJSON), time.Now())
		}
	} else {
		_, execErr = pool.Exec(ctx, `
UPDATE agent_runs SET status=$2, output=$3::jsonb, error=NULL, completed_at=NULL
WHERE id=$1::uuid`, runID, status, string(outJSON))
	}
	return Result{Status: status, Output: output, Error: errMsg}, execErr
}

func loadOutput(ctx context.Context, pool *db.Pool, runID string) map[string]any {
	var raw []byte
	err := pool.QueryRow(ctx, `SELECT COALESCE(output, '{}'::jsonb) FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&raw)
	if err != nil || len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func mergeOutput(dst, src map[string]any) {
	for k, v := range src {
		if _, exists := dst[k]; exists {
			continue
		}
		dst[k] = v
	}
}

func pendingFromOutput(out map[string]any) (PendingToolCall, bool) {
	if out == nil {
		return PendingToolCall{}, false
	}
	raw, ok := out["pendingToolCall"]
	if !ok {
		return PendingToolCall{}, false
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return PendingToolCall{}, false
	}
	var p PendingToolCall
	if json.Unmarshal(b, &p) != nil || strings.TrimSpace(p.Name) == "" {
		return PendingToolCall{}, false
	}
	if p.Arguments == nil {
		p.Arguments = map[string]any{}
	}
	return p, true
}

func remainingFromOutput(out map[string]any) []inference.ToolCall {
	if out == nil {
		return nil
	}
	raw, ok := out["remainingToolCalls"]
	if !ok {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var calls []inference.ToolCall
	if json.Unmarshal(b, &calls) != nil {
		return nil
	}
	return calls
}

func messagesFromOutput(out map[string]any) []inference.Message {
	if out == nil {
		return nil
	}
	raw, ok := out["loopTranscript"]
	if !ok {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var msgs []inference.Message
	if json.Unmarshal(b, &msgs) != nil {
		return nil
	}
	return msgs
}

func intFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
