package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/agentloop"
	"github.com/MajestaNet/ide/internal/inference"
)

func (s *Server) inferenceResolveOpts() inference.ResolveOptions {
	opts := inference.ResolveOptions{EncKey: s.encKey()}
	if s.cfg != nil {
		opts.DOAPIToken = s.cfg.DigitalOceanAPIToken
		opts.AllowDevLocal = !s.cfg.IsProduction
	}
	return opts
}

func (s *Server) allowDevLocalInference() bool {
	return s.cfg != nil && !s.cfg.IsProduction
}

func wantsAgentStream(r *http.Request, streamFlag bool) bool {
	if streamFlag {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/event-stream")
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(b)); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

type agentLoopHTTPInput struct {
	runID, goal, playbook string
	input                 map[string]any
	dryRun, resume        bool
	applied               agentharness.Applied
	skills, scopes        []string
}

// streamAgentRunLLM runs the hosted tool loop in-process and streams SSE events.
func (s *Server) streamAgentRunLLM(w http.ResponseWriter, r *http.Request, in agentLoopHTTPInput) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "STREAM_UNSUPPORTED", "streaming not supported")
		return
	}
	pool := s.pool
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	_, _ = agentloop.Execute(r.Context(), agentloop.Config{
		Pool:            pool,
		MCP:             s.mcpDeps(),
		Applied:         in.applied,
		AllowedSkills:   in.skills,
		ObjectScopes:    in.scopes,
		PlaybookAPIName: in.playbook,
		ResolveOpts:     s.inferenceResolveOpts(),
		Stream:          true,
	}, agentloop.Input{
		RunID:  in.runID,
		Goal:   in.goal,
		Input:  in.input,
		DryRun: in.dryRun,
		Resume: in.resume,
	}, func(event string, payload any) {
		_, _ = inference.AppendRunEvent(r.Context(), pool, in.runID, event, payload)
		_ = writeSSE(w, flusher, event, payload)
	})
}

func (s *Server) handleStreamAgentRun(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "STREAM_UNSUPPORTED", "streaming not supported")
		return
	}
	runID := r.PathValue("id")
	afterSeq := 0
	if v := r.URL.Query().Get("afterSeq"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			afterSeq = n
		}
	}
	var status string
	var actorID *string
	err := pool.QueryRow(r.Context(), `SELECT status, actor_id::text FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&status, &actorID)
	if err != nil || !s.canReadAgentRun(r, actorID) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Agent run not found")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(5 * time.Minute)
	for {
		events, err := inference.ListRunEventsAfter(r.Context(), pool, runID, afterSeq)
		if err != nil {
			_ = writeSSE(w, flusher, "error", map[string]any{"error": err.Error()})
			return
		}
		for _, e := range events {
			_ = writeSSE(w, flusher, e.Type, e.Payload)
			afterSeq = e.Seq
		}
		_ = pool.QueryRow(r.Context(), `SELECT status FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&status)
		if status == "completed" || status == "failed" || status == "dry_run_complete" || status == agentloop.StatusAwaitingToolApproval {
			if len(events) == 0 {
				// Ensure terminal signal for clients that missed events.
				_ = writeSSE(w, flusher, "done", map[string]any{"id": runID, "status": status})
			}
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-deadline:
			_ = writeSSE(w, flusher, "error", map[string]any{"error": "stream timeout"})
			return
		case <-ticker.C:
		}
	}
}
