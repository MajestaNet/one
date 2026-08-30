// Package agentloop is the hosted /agents/runs tool executor (BP-006).
// Worker JSON jobs and in-process SSE share this package. Tools execute via mcp.CallTool
// as the reconstructed run actor. Control IDE graphCalls are persisted but never Applied.
package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/inference"
	"github.com/MajestaNet/ide/internal/mcp"
)

const (
	// StatusAwaitingToolApproval parks a gated write until POST .../approve.
	StatusAwaitingToolApproval = "awaiting_tool_approval"
	StatusRunning              = "running"
	StatusCompleted            = "completed"
	StatusFailed               = "failed"
	StatusDryRunComplete       = "dry_run_complete"

	// MaxToolRounds is the v1 model→tool round cap (retune without schema).
	MaxToolRounds = 8
	// MaxToolResultBytes truncates tool results in the model transcript (32 KiB).
	MaxToolResultBytes = 32 * 1024

	EventToolCall         = "tool_call"
	EventToolResult       = "tool_result"
	EventApprovalRequired = "approval_required"
	EventRun              = "run"
	EventHarness          = "harness"
	EventToken            = "token"
	EventDone             = "done"
	EventError            = "error"
)

// Sink receives SSE-adjacent events (persist + optional live stream).
type Sink func(event string, payload any)

// Config is shared by worker and HTTP SSE callers.
type Config struct {
	Pool              *db.Pool
	MCP               mcp.Deps
	Applied           agentharness.Applied
	AllowedSkills     []string
	ObjectScopes      []string
	PlaybookAPIName   string
	ResolveOpts       inference.ResolveOptions
	Stream            bool
	SoftSkipInference bool // worker: stub-complete when inference is not configured
}

// Input is one Execute invocation (fresh generation or approve resume).
type Input struct {
	RunID  string
	Goal   string
	Input  map[string]any
	DryRun bool
	Resume bool
}

// PendingToolCall is persisted on agent_runs.output when a write is parked.
type PendingToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Round     int            `json:"round"`
}

// Result is the terminal or parked outcome.
type Result struct {
	Status string
	Output map[string]any
	Error  string
}

var errMissingActor = errors.New("agent loop: run actor_id is missing or unresolvable")

// Execute runs the hosted tool loop. AuthZ is the reconstructed run Actor, never DEFAULT_OWNER_ID.
func Execute(ctx context.Context, cfg Config, in Input, sink Sink) (Result, error) {
	if in.Input == nil {
		in.Input = map[string]any{}
	}
	emit := func(event string, payload any) {
		if sink != nil {
			sink(event, payload)
		}
	}

	actor, actorID, err := reconstructActor(ctx, cfg.Pool, in.RunID)
	if err != nil {
		res := failRun(ctx, cfg.Pool, in.RunID, err.Error(), nil)
		emit(EventError, map[string]any{"code": "ACTOR_REQUIRED", "error": err.Error()})
		return res, err
	}

	admitted := agentharness.ExpandToHostedMCP(cfg.Applied.AllowedTools)
	harnessMeta := cfg.Applied.Meta()
	output := map[string]any{
		"goal":              in.Goal,
		"toolsPlanned":      cfg.Applied.AllowedTools,
		"admittedTools":     admitted,
		"objectScopes":      cfg.ObjectScopes,
		"skillsGranted":     cfg.AllowedSkills,
		"allowlistEnforced": true,
		"harness":           harnessMeta,
		"actorId":           actorID,
	}

	_, _ = cfg.Pool.Exec(ctx, `UPDATE agent_runs SET status=$2 WHERE id=$1::uuid`, in.RunID, StatusRunning)
	if !in.Resume {
		emit(EventRun, map[string]any{"id": in.RunID, "status": StatusRunning})
		emit(EventHarness, harnessMeta)
	} else {
		emit(EventRun, map[string]any{"id": in.RunID, "status": StatusRunning, "resume": true})
	}

	if in.DryRun {
		return executeDryRun(ctx, cfg, in, admitted, output, emit)
	}

	priorOut := loadOutput(ctx, cfg.Pool, in.RunID)
	msgs := inference.BuildAgentMessages(cfg.Applied.SystemInstructions, in.Goal, in.Input)
	round := 0
	if in.Resume {
		if prior := messagesFromOutput(priorOut); len(prior) > 0 {
			msgs = prior
		}
		if n, ok := intFromAny(priorOut["loopRound"]); ok {
			round = n
		}
		pending, ok := pendingFromOutput(priorOut)
		if !ok {
			res := failRun(ctx, cfg.Pool, in.RunID, "resume requested but no pendingToolCall", output)
			emit(EventError, map[string]any{"error": res.Error})
			return res, fmt.Errorf("agent loop: no pendingToolCall")
		}
		mergeOutput(output, priorOut)
		toolMsgs, execErr := executeOne(ctx, cfg, actor, admitted, pending.Name, pending.ID, pending.Arguments, emit)
		if execErr != nil {
			res := failRun(ctx, cfg.Pool, in.RunID, execErr.Error(), output)
			emit(EventError, map[string]any{"code": "TOOL_DENIED", "error": execErr.Error()})
			return res, execErr
		}
		msgs = append(msgs, toolMsgs...)
		delete(output, "pendingToolCall")
		if rest := remainingFromOutput(priorOut); len(rest) > 0 {
			extra, park, perr := processCalls(ctx, cfg, actor, admitted, rest, round, emit)
			if perr != nil {
				res := failRun(ctx, cfg.Pool, in.RunID, perr.Error(), output)
				emit(EventError, map[string]any{"code": "TOOL_DENIED", "error": perr.Error()})
				return res, perr
			}
			msgs = append(msgs, extra...)
			if park != nil {
				return parkRun(ctx, cfg.Pool, in.RunID, output, msgs, *park, emit)
			}
		}
		round++
	}

	route, rerr := inference.Resolve(ctx, cfg.Pool, cfg.ResolveOpts)
	if rerr != nil {
		code, msg := inference.FormatRouteError(rerr)
		if cfg.SoftSkipInference && (errors.Is(rerr, inference.ErrNotConfigured) || errors.Is(rerr, inference.ErrDOTokenMissing)) {
			output["summary"] = "Worker completed agent run (inference not configured — " + msg + ")"
			output["inferenceSkipped"] = true
			return completeRun(ctx, cfg.Pool, in.RunID, StatusCompleted, output, emit)
		}
		res := failRun(ctx, cfg.Pool, in.RunID, msg, output)
		emit(EventError, map[string]any{"code": code, "error": msg})
		return res, rerr
	}
	output["model"] = route.Model
	output["source"] = string(route.Source)
	if route.BillingNotice != "" {
		output["billingNotice"] = route.BillingNotice
	}

	tools := chatTools(admitted)
	for round < MaxToolRounds {
		text, calls, gerr := generate(ctx, cfg, route, msgs, tools, emit)
		if gerr != nil {
			res := failRun(ctx, cfg.Pool, in.RunID, gerr.Error(), output)
			emit(EventError, map[string]any{"code": "INFERENCE_ERROR", "error": gerr.Error()})
			return res, gerr
		}
		if len(calls) == 0 {
			enriched := inference.EnrichAgentOutput(output, text)
			for k, v := range enriched {
				output[k] = v
			}
			return completeRun(ctx, cfg.Pool, in.RunID, StatusCompleted, output, emit)
		}
		assistant := inference.Message{Role: "assistant", Content: text, ToolCalls: calls}
		msgs = append(msgs, assistant)
		toolMsgs, park, perr := processCalls(ctx, cfg, actor, admitted, calls, round, emit)
		if perr != nil {
			res := failRun(ctx, cfg.Pool, in.RunID, perr.Error(), output)
			emit(EventError, map[string]any{"code": "TOOL_DENIED", "error": perr.Error()})
			return res, perr
		}
		msgs = append(msgs, toolMsgs...)
		if park != nil {
			return parkRun(ctx, cfg.Pool, in.RunID, output, msgs, *park, emit)
		}
		round++
	}
	output["summary"] = fmt.Sprintf("Stopped after %d tool rounds", MaxToolRounds)
	return completeRun(ctx, cfg.Pool, in.RunID, StatusCompleted, output, emit)
}

func executeDryRun(ctx context.Context, cfg Config, in Input, admitted []string, output map[string]any, emit Sink) (Result, error) {
	route, rerr := inference.Resolve(ctx, cfg.Pool, cfg.ResolveOpts)
	if rerr != nil {
		output["summary"] = "Dry-run: planned Platform API tool calls only"
		if cfg.Stream && !cfg.SoftSkipInference {
			code, msg := inference.FormatRouteError(rerr)
			if !errors.Is(rerr, inference.ErrNotConfigured) && !errors.Is(rerr, inference.ErrDOTokenMissing) {
				res := failRun(ctx, cfg.Pool, in.RunID, msg, output)
				emit(EventError, map[string]any{"code": code, "error": msg})
				return res, rerr
			}
		}
		return completeRun(ctx, cfg.Pool, in.RunID, StatusDryRunComplete, output, emit)
	}
	msgs := inference.BuildAgentMessages(cfg.Applied.SystemInstructions, in.Goal, in.Input)
	text, calls, gerr := generate(ctx, cfg, route, msgs, chatTools(admitted), emit)
	if gerr != nil {
		output["summary"] = "Dry-run: planned Platform API tool calls only"
		output["inferenceError"] = gerr.Error()
		return completeRun(ctx, cfg.Pool, in.RunID, StatusDryRunComplete, output, emit)
	}
	planned := make([]map[string]any, 0, len(calls))
	for _, c := range calls {
		emit(EventToolCall, redactToolCallPayload(c.Function.Name, c.ArgsMap()))
		planned = append(planned, map[string]any{"name": c.Function.Name, "arguments": redactArgs(c.ArgsMap())})
	}
	output["summary"] = "Dry-run: planned Platform API tool calls only"
	output["plannedToolCalls"] = planned
	if text != "" {
		output["modelText"] = text
	}
	output["model"] = route.Model
	output["source"] = string(route.Source)
	return completeRun(ctx, cfg.Pool, in.RunID, StatusDryRunComplete, output, emit)
}

func generate(ctx context.Context, cfg Config, route *inference.Route, msgs []inference.Message, tools []inference.Tool, emit Sink) (string, []inference.ToolCall, error) {
	req := inference.ChatRequest{Messages: msgs, Tools: tools}
	if cfg.Stream {
		client := inference.StreamClientForRoute(route)
		var full strings.Builder
		var assembled []inference.ToolCall
		err := client.Stream(ctx, route, req, func(chunk inference.StreamChunk) error {
			if chunk.Done {
				if len(chunk.ToolCalls) > 0 {
					assembled = chunk.ToolCalls
				}
				return nil
			}
			if chunk.Delta != "" {
				full.WriteString(chunk.Delta)
				payload := map[string]any{"delta": chunk.Delta}
				emit(EventToken, payload)
			}
			if len(chunk.ToolCalls) > 0 {
				assembled = chunk.ToolCalls
			}
			return nil
		})
		if err != nil {
			return "", nil, err
		}
		text := full.String()
		calls := assembled
		if len(calls) == 0 {
			_, effects := inference.ParseStructuredAgentOutput(text)
			calls = inference.ToolCallsFromEffects(effects)
		}
		return text, calls, nil
	}
	client := inference.ClientForRoute(route)
	resp, err := client.Complete(ctx, route, req)
	if err != nil {
		return "", nil, err
	}
	text := inference.TextContent(resp)
	calls := inference.ResponseToolCalls(resp)
	if len(calls) == 0 {
		_, effects := inference.ParseStructuredAgentOutput(text)
		calls = inference.ToolCallsFromEffects(effects)
	}
	return text, calls, nil
}

func processCalls(ctx context.Context, cfg Config, actor *authz.Actor, admitted []string, calls []inference.ToolCall, round int, emit Sink) ([]inference.Message, *pendingPark, error) {
	var msgs []inference.Message
	for i, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		args := call.ArgsMap()
		id := strings.TrimSpace(call.ID)
		if id == "" {
			id = fmt.Sprintf("call-%d-%d", round, i+1)
		}
		emit(EventToolCall, redactToolCallPayload(name, args))
		if !agentharness.HostedToolAdmitted(name, admitted) {
			return msgs, nil, fmt.Errorf("tool %q is not in the hosted allowlist", name)
		}
		class := agentharness.HostedToolClass(name)
		if class == "" {
			return msgs, nil, fmt.Errorf("tool %q is not in the hosted v1 catalog", name)
		}
		if class == agentharness.ToolClassWrite && cfg.Applied.RequireApproval {
			remaining := append([]inference.ToolCall(nil), calls[i+1:]...)
			return msgs, &pendingPark{
				Call:      PendingToolCall{ID: id, Name: name, Arguments: args, Round: round},
				Remaining: remaining,
			}, nil
		}
		toolMsgs, err := executeOne(ctx, cfg, actor, admitted, name, id, args, emit)
		if err != nil {
			return msgs, nil, err
		}
		msgs = append(msgs, toolMsgs...)
	}
	return msgs, nil, nil
}

type pendingPark struct {
	Call      PendingToolCall
	Remaining []inference.ToolCall
}

func executeOne(ctx context.Context, cfg Config, actor *authz.Actor, admitted []string, name, callID string, args map[string]any, emit Sink) ([]inference.Message, error) {
	if !agentharness.HostedToolAdmitted(name, admitted) {
		return nil, fmt.Errorf("tool %q is not in the hosted allowlist", name)
	}
	if err := assertObjectScope(args, cfg.ObjectScopes); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	if name == "invoke_skill" && strings.TrimSpace(stringify(args["playbookApiName"])) == "" && cfg.PlaybookAPIName != "" {
		args["playbookApiName"] = cfg.PlaybookAPIName
	}
	result, err := mcp.CallTool(ctx, cfg.MCP, actor, name, args)
	if err != nil {
		if errors.Is(err, mcp.ErrUnauthorized) || errors.Is(err, mcp.ErrForbidden) {
			return nil, err
		}
		payload := map[string]any{"name": name, "error": err.Error()}
		emit(EventToolResult, payload)
		content := truncateForModel(err.Error())
		return []inference.Message{{Role: "tool", ToolCallID: callID, Name: name, Content: content}}, nil
	}
	full, _ := json.Marshal(result)
	emit(EventToolResult, map[string]any{"name": name, "result": jsonLimit(result)})
	return []inference.Message{{
		Role:       "tool",
		ToolCallID: callID,
		Name:       name,
		Content:    truncateForModel(string(full)),
	}}, nil
}

func assertObjectScope(args map[string]any, scopes []string) error {
	if len(scopes) == 0 || args == nil {
		return nil
	}
	obj := strings.TrimSpace(stringify(args["object"]))
	if obj == "" {
		obj = strings.TrimSpace(stringify(args["objectApiName"]))
	}
	if obj == "" {
		return nil
	}
	for _, s := range scopes {
		if strings.EqualFold(strings.TrimSpace(s), obj) {
			return nil
		}
	}
	return fmt.Errorf("object %s is not in playbook objectScopes", obj)
}

func reconstructActor(ctx context.Context, pool *db.Pool, runID string) (*authz.Actor, string, error) {
	var actorID *string
	err := pool.QueryRow(ctx, `SELECT actor_id::text FROM agent_runs WHERE id=$1::uuid`, runID).Scan(&actorID)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", errMissingActor, err)
	}
	if actorID == nil || strings.TrimSpace(*actorID) == "" {
		return nil, "", errMissingActor
	}
	users := &db.AuthzUsers{Store: db.NewUserStore(pool)}
	actor, err := authz.LoadActor(ctx, users, *actorID)
	if err != nil {
		return nil, *actorID, fmt.Errorf("%w: %v", errMissingActor, err)
	}
	return actor, actor.ID, nil
}

func chatTools(admitted []string) []inference.Tool {
	allow := map[string]struct{}{}
	for _, n := range admitted {
		allow[n] = struct{}{}
	}
	var out []inference.Tool
	for _, t := range mcp.ListTools() {
		if _, ok := allow[t.Name]; !ok {
			continue
		}
		out = append(out, inference.Tool{
			Type: "function",
			Function: inference.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return out
}
