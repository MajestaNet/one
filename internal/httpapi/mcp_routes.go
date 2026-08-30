package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/mcp"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/version"
)

func (s *Server) registerMCPRoutes() {
	if s.cfg == nil || !s.cfg.AgentsEnabled() {
		return
	}
	// Streamable HTTP MCP endpoint (stateless JSON). GET/DELETE return 405 — no SSE/sessions in v1.
	s.mux.Handle("POST /mcp", s.requireAuth(http.HandlerFunc(s.handleMCP)))
	s.mux.Handle("GET /mcp", s.requireAuth(http.HandlerFunc(s.handleMCPGet)))
	s.mux.Handle("DELETE /mcp", s.requireAuth(http.HandlerFunc(s.handleMCPDelete)))
	// Convenience authenticated catalog (non-JSON-RPC).
	s.mux.Handle("GET /mcp/tools", s.requireAuth(http.HandlerFunc(s.handleMCPListTools)))
}

func (s *Server) mcpDeps() mcp.Deps {
	productVersion := ""
	if s.cfg != nil {
		productVersion = s.cfg.ProductVersion
	}
	return mcp.Deps{
		Meta:           s.meta,
		Data:           s.data,
		Pool:           s.pool,
		ObjectAz:       s.objectAz,
		FieldAz:        s.fieldAz,
		RecordAccess:   s.recordAccess,
		Actions:        s.actions,
		Deploy:         s.deploy,
		SystemAz:       s.systemAz,
		AutomationAz:   s.automationAz,
		ProductVersion: productVersion,
		Version:        version.Version,
	}
}

func (s *Server) handleMCPListTools(w http.ResponseWriter, r *http.Request) {
	if !mcpAllowOrigin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": mcp.ListTools()})
}

func (s *Server) handleMCPGet(w http.ResponseWriter, r *http.Request) {
	if !mcpAllowOrigin(w, r) {
		return
	}
	// Stateless JSON mode: no standalone SSE listen stream.
	w.Header().Set("Allow", "POST")
	writeErr(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "MCP SSE listen is not enabled; use POST /mcp with Accept: application/json")
}

func (s *Server) handleMCPDelete(w http.ResponseWriter, r *http.Request) {
	if !mcpAllowOrigin(w, r) {
		return
	}
	w.Header().Set("Allow", "POST")
	writeErr(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "MCP sessions are not used; this gateway is stateless")
}

// handleMCP implements Streamable HTTP POST: JSON-RPC requests return application/json;
// notifications/responses-only bodies return 202 Accepted.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if !mcpAllowOrigin(w, r) {
		return
	}
	if !mcpAcceptOK(r) {
		writeErr(w, http.StatusNotAcceptable, "NOT_ACCEPTABLE", "Accept must include application/json and/or text/event-stream")
		return
	}

	body, err := readBodyLimited(r.Body, 1<<20)
	if err != nil {
		if requestBodyTooLarge(err) {
			writeErr(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unable to read body")
		return
	}
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Empty body")
		return
	}

	actor := ActorFromContext(r.Context())

	// Batch array
	if body[0] == '[' {
		var msgs []json.RawMessage
		if err := json.Unmarshal(body, &msgs); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON batch")
			return
		}
		var responses []any
		onlyNotify := true
		batchStatus := http.StatusOK
		for _, raw := range msgs {
			resp, isReq, httpStatus, dErr := s.dispatchMCPMessage(r, actor, raw)
			if dErr != nil {
				writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", dErr.Error())
				return
			}
			if isReq {
				onlyNotify = false
				if resp != nil {
					responses = append(responses, resp)
				}
				if httpStatus > batchStatus {
					batchStatus = httpStatus
				}
			}
		}
		if onlyNotify {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("MCP-Protocol-Version", mcp.LatestProtocolVersion)
		w.WriteHeader(batchStatus)
		_ = json.NewEncoder(w).Encode(responses)
		return
	}

	resp, isReq, httpStatus, dErr := s.dispatchMCPMessage(r, actor, body)
	if dErr != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", dErr.Error())
		return
	}
	if !isReq {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if httpStatus == 0 {
		httpStatus = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("MCP-Protocol-Version", mcp.LatestProtocolVersion)
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(resp)
}

type mcpRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

// dispatchMCPMessage handles one JSON-RPC object.
// isReq is true when the message is a request that needs a JSON-RPC response body.
// httpStatus is the HTTP status for the response (0 → 200); AuthZ denials keep 401/403.
func (s *Server) dispatchMCPMessage(r *http.Request, actor *authz.Actor, raw json.RawMessage) (resp any, isReq bool, httpStatus int, err error) {
	var msg mcpRPC
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, false, 0, errors.New("invalid JSON-RPC object")
	}
	if msg.JSONRPC != "" && msg.JSONRPC != "2.0" {
		return nil, false, 0, errors.New("jsonrpc must be \"2.0\"")
	}

	// Client responses (result or error, no method) — accept with 202.
	if msg.Method == "" {
		return nil, false, 0, nil
	}

	if mcp.IsNotification(msg.Method) {
		return nil, false, 0, nil
	}

	switch msg.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(msg.Params) > 0 {
			_ = json.Unmarshal(msg.Params, &params)
		}
		ver := mcp.NegotiateProtocolVersion(params.ProtocolVersion)
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      msg.ID,
			"result":  mcp.InitializeResult(ver),
		}, true, http.StatusOK, nil

	case "ping":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      msg.ID,
			"result":  map[string]any{},
		}, true, http.StatusOK, nil

	case "tools/list":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      msg.ID,
			"result":  map[string]any{"tools": mcp.ListTools()},
		}, true, http.StatusOK, nil

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if len(msg.Params) > 0 {
			_ = json.Unmarshal(msg.Params, &params)
		}
		result, callErr := mcp.CallTool(r.Context(), s.mcpDeps(), actor, params.Name, params.Arguments)
		if callErr != nil {
			code, apiCode, message := mcpError(callErr)
			return map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"error": map[string]any{
					"code":    jsonRPCCode(code),
					"message": message,
					"data":    map[string]any{"httpStatus": code, "apiCode": apiCode},
				},
			}, true, code, nil
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      msg.ID,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": mustJSON(result)},
				},
			},
		}, true, http.StatusOK, nil

	default:
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      msg.ID,
			"error": map[string]any{
				"code":    -32601,
				"message": "Method not found: " + msg.Method,
			},
		}, true, http.StatusBadRequest, nil
	}
}

func mcpError(err error) (httpStatus int, code, message string) {
	message = err.Error()
	var be *deploy.BusyError
	if errors.As(err, &be) && be != nil {
		return http.StatusTooManyRequests, "DEPLOY_BUSY", be.Error()
	}
	if errors.Is(err, deploy.ErrBusy) {
		return http.StatusTooManyRequests, "DEPLOY_BUSY", err.Error()
	}
	var ae *actions.Error
	if errors.As(err, &ae) && ae != nil {
		return ae.Status, ae.Code, ae.Message
	}
	switch {
	case errors.Is(err, mcp.ErrUnauthorized):
		return http.StatusUnauthorized, "UNAUTHORIZED", message
	case errors.Is(err, mcp.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN", message
	case errors.Is(err, mcp.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND", message
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, metadata.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN", message
	default:
		return http.StatusBadRequest, "TOOL_ERROR", message
	}
}

func jsonRPCCode(httpStatus int) int {
	switch httpStatus {
	case http.StatusUnauthorized, http.StatusForbidden:
		return -32000
	case http.StatusNotFound:
		return -32601
	default:
		return -32602
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// mcpAcceptOK implements Streamable HTTP Accept requirements loosely:
// allow application/json and/or text/event-stream; also allow */* or empty for curl/tests.
func mcpAcceptOK(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" || strings.Contains(accept, "*/*") {
		return true
	}
	return strings.Contains(accept, "application/json") || strings.Contains(accept, "text/event-stream")
}

// mcpAllowOrigin mitigates DNS rebinding for loopback Hosts: if Host is loopback and
// Origin is present and non-loopback, reject. Authenticated remote installs allow any Origin.
func mcpAllowOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}
	if !mcp.IsLoopbackHost(host) {
		return true
	}
	o := strings.TrimPrefix(origin, "https://")
	o = strings.TrimPrefix(o, "http://")
	if i := strings.IndexAny(o, "/:"); i >= 0 {
		o = o[:i]
	}
	if mcp.IsLoopbackHost(o) || origin == "null" {
		return true
	}
	writeErr(w, http.StatusForbidden, "FORBIDDEN", "Origin not allowed for loopback MCP endpoint")
	return false
}
