package automation

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed embed/bootstrap.ts
var bootstrapTS string

//go:embed embed/one_automation.ts
var oneAutomationTS string

// DenoVersion is the pinned guest runtime (install-side executor only).
const DenoVersion = "2.9.3"

// DefaultAsyncDeadline caps async automation.run Deno wall time.
const DefaultAsyncDeadline = 30 * time.Second

// ErrDenoUnavailable is returned when the Deno binary cannot be found.
var ErrDenoUnavailable = errors.New("deno binary not found (set DENO_PATH or install Deno " + DenoVersion + ")")

// HostBridge is the Go-side SDK implementation for guest ctx.* calls.
type HostBridge interface {
	CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (recordID string, err error)
	UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error
	GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error)
	DeleteRecord(ctx context.Context, objectAPIName, recordID string) error
	Query(ctx context.Context, req map[string]any) (map[string]any, error)
	InvokeAction(ctx context.Context, apiName string, input map[string]any) (map[string]any, error)
}

// GuestRequest is one Deno guest invocation.
type GuestRequest struct {
	APIName   string
	Source    string
	EntryFile string
	Trigger   SyncTrigger
	Timeout   time.Duration
	// SyncMode selects the sync wall-clock default when Timeout is unset.
	SyncMode bool
	// DenoPath overrides binary discovery when non-empty.
	DenoPath string
	// Logger receives guest ctx.log lines (optional).
	Logger func(message string)
	// Bootstrap overrides the embedded runtime bootstrap (unit harness uses a variant).
	Bootstrap string
	// ExtraFiles are written into the guest workdir (e.g. under_test.ts).
	ExtraFiles map[string]string
}

// GuestResult is the guest's returned value (usually {ok:true}).
type GuestResult struct {
	OK   bool
	Raw  map[string]any
	Logs []string
}

// FindDeno returns the Deno binary path (DENO_PATH, then PATH).
func FindDeno(explicit string) (string, error) {
	if explicit != "" {
		if st, err := os.Stat(explicit); err == nil && !st.IsDir() {
			return explicit, nil
		}
		return "", fmt.Errorf("%w: %s", ErrDenoUnavailable, explicit)
	}
	if p := os.Getenv("DENO_PATH"); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("deno"); err == nil {
		return p, nil
	}
	return "", ErrDenoUnavailable
}

// RunGuest executes customer TypeScript under Deno default-deny with host RPC.
func RunGuest(ctx context.Context, host HostBridge, req GuestRequest) (*GuestResult, error) {
	if host == nil {
		return nil, fmt.Errorf("automation %s: nil host bridge", req.APIName)
	}
	if req.SyncMode {
		host = SyncOutboundBan{Inner: host}
	}
	if strings.TrimSpace(req.Source) == "" {
		return nil, fmt.Errorf("automation %s: empty source", req.APIName)
	}
	entryLabel := req.EntryFile
	if entryLabel == "" {
		entryLabel = "src/automations/" + req.APIName + ".ts"
	}
	if err := ValidateSourceImports(entryLabel, req.Source); err != nil {
		return nil, err
	}

	denoPath, err := FindDeno(req.DenoPath)
	if err != nil {
		return nil, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		if req.SyncMode {
			timeout = SyncDeadline
		} else {
			timeout = DefaultAsyncDeadline
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workDir, err := os.MkdirTemp("", "one-auto-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	if err := materializeGuestWorkdir(workDir, req); err != nil {
		return nil, err
	}

	args := []string{
		"run",
		"--no-npm",
		"--no-remote",
		"--quiet",
		"--allow-read=" + workDir,
		"--import-map=" + filepath.Join(workDir, "import_map.json"),
		filepath.Join(workDir, "bootstrap.ts"),
	}
	cmd := exec.CommandContext(runCtx, denoPath, args...)
	cmd.Dir = workDir
	cmd.Env = []string{
		"HOME=" + workDir,
		"DENO_DIR=" + filepath.Join(workDir, ".deno"),
		"NO_COLOR=1",
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("automation %s: start deno: %w", req.APIName, err)
	}

	result, runErr := serveGuestRPC(runCtx, host, req, stdin, stdout)
	waitErr := cmd.Wait()
	_ = stdin.Close()

	if runErr != nil {
		if stderrBuf.Len() > 0 {
			return nil, fmt.Errorf("%w\ndeno stderr: %s", runErr, strings.TrimSpace(stderrBuf.String()))
		}
		return nil, runErr
	}
	if waitErr != nil && result == nil {
		if runCtx.Err() != nil {
			return nil, fmt.Errorf("automation %s: deno deadline exceeded: %w", req.APIName, runCtx.Err())
		}
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return nil, fmt.Errorf("automation %s: deno exited: %s", req.APIName, msg)
	}
	if result == nil {
		return nil, fmt.Errorf("automation %s: deno produced no result", req.APIName)
	}
	return result, nil
}

func materializeGuestWorkdir(workDir string, req GuestRequest) error {
	boot := req.Bootstrap
	if boot == "" {
		boot = bootstrapTS
	}
	files := map[string][]byte{
		"bootstrap.ts":      []byte(boot),
		"one_automation.ts": []byte(oneAutomationTS),
		"user_entry.ts":     []byte(req.Source),
		"import_map.json":   []byte(`{"imports":{"one:automation":"./one_automation.ts"}}`),
	}
	for name, body := range req.ExtraFiles {
		if name == "" {
			continue
		}
		files[name] = []byte(body)
	}
	trig, err := json.Marshal(map[string]any{
		"action":        req.Trigger.Action,
		"objectApiName": req.Trigger.ObjectAPIName,
		"recordId":      req.Trigger.RecordID,
		"data":          req.Trigger.Data,
		"actorId":       req.Trigger.ActorID,
	})
	if err != nil {
		return err
	}
	files["trigger.json"] = trig
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(workDir, name), body, 0o600); err != nil {
			return err
		}
	}
	return nil
}

type rpcIn struct {
	Kind    string          `json:"kind"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Args    json.RawMessage `json:"args"`
	Result  json.RawMessage `json:"result"`
	Error   string          `json:"error"`
	Message string          `json:"message"`
}

func serveGuestRPC(ctx context.Context, host HostBridge, req GuestRequest, stdin io.WriteCloser, stdout io.Reader) (*GuestResult, error) {
	var (
		mu   sync.Mutex
		logs []string
	)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	writeResp := func(id int, value any, errMsg string) error {
		msg := map[string]any{"kind": "rpcResult", "id": id}
		if errMsg != "" {
			msg["error"] = errMsg
		} else {
			msg["result"] = value
		}
		b, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		_, err = stdin.Write(append(b, '\n'))
		return err
	}

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("automation %s: aborted: %w", req.APIName, err)
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg rpcIn
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		switch msg.Kind {
		case "log":
			logs = append(logs, msg.Message)
			if req.Logger != nil {
				req.Logger(msg.Message)
			}
		case "error":
			return nil, fmt.Errorf("automation %s: %s", req.APIName, msg.Error)
		case "result":
			raw := map[string]any{}
			_ = json.Unmarshal(msg.Result, &raw)
			ok := true
			if v, exists := raw["ok"]; exists {
				if b, isBool := v.(bool); isBool {
					ok = b
				}
			}
			if errStr, _ := raw["error"].(string); errStr != "" {
				return nil, fmt.Errorf("automation %s: %s", req.APIName, errStr)
			}
			if !ok {
				return nil, fmt.Errorf("automation %s: guest returned ok=false", req.APIName)
			}
			return &GuestResult{OK: ok, Raw: raw, Logs: append([]string{}, logs...)}, nil
		case "rpc":
			val, err := dispatchHostRPC(ctx, host, msg.Method, msg.Args)
			if err != nil {
				if werr := writeResp(msg.ID, nil, err.Error()); werr != nil {
					return nil, werr
				}
				continue
			}
			if werr := writeResp(msg.ID, val, ""); werr != nil {
				return nil, werr
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("automation %s: read deno stdout: %w", req.APIName, err)
	}
	return nil, nil
}

func dispatchHostRPC(ctx context.Context, host HostBridge, method string, argsJSON json.RawMessage) (any, error) {
	method = strings.TrimSpace(method)
	if u, ok := host.(UnitRPCHandler); ok {
		if result, handled, err := u.HandleUnitRPC(ctx, method, argsJSON); handled {
			return result, err
		}
	}
	switch method {
	case "createRecord":
		var args struct {
			ObjectAPIName string         `json:"objectApiName"`
			Data          map[string]any `json:"data"`
		}
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		if args.ObjectAPIName == "" {
			return nil, fmt.Errorf("createRecord requires objectApiName")
		}
		if args.Data == nil {
			args.Data = map[string]any{}
		}
		id, err := host.CreateRecord(ctx, args.ObjectAPIName, args.Data)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id}, nil
	case "updateRecord":
		var args struct {
			ObjectAPIName string         `json:"objectApiName"`
			RecordID      string         `json:"recordId"`
			Data          map[string]any `json:"data"`
		}
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		if args.ObjectAPIName == "" || args.RecordID == "" {
			return nil, fmt.Errorf("updateRecord requires objectApiName and recordId")
		}
		if err := host.UpdateRecord(ctx, args.ObjectAPIName, args.RecordID, args.Data); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "getRecord":
		var args struct {
			ObjectAPIName string `json:"objectApiName"`
			RecordID      string `json:"recordId"`
		}
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		rec, err := host.GetRecord(ctx, args.ObjectAPIName, args.RecordID)
		if err != nil {
			return nil, err
		}
		return rec, nil
	case "deleteRecord":
		var args struct {
			ObjectAPIName string `json:"objectApiName"`
			RecordID      string `json:"recordId"`
		}
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		if err := host.DeleteRecord(ctx, args.ObjectAPIName, args.RecordID); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "query":
		var args map[string]any
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		return host.Query(ctx, args)
	case "http":
		ob, ok := host.(OutboundBridge)
		if !ok {
			return nil, fmt.Errorf("http is not available on this host")
		}
		var args HTTPCallArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		return ob.HTTPCall(ctx, args)
	case "connector":
		ob, ok := host.(OutboundBridge)
		if !ok {
			return nil, fmt.Errorf("connector is not available on this host")
		}
		var args ConnectorCallArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		return ob.ConnectorCall(ctx, args)
	case "invokeAction":
		var args struct {
			APIName string         `json:"apiName"`
			Input   map[string]any `json:"input"`
		}
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		if strings.TrimSpace(args.APIName) == "" {
			return nil, fmt.Errorf("invokeAction requires apiName")
		}
		if args.Input == nil {
			args.Input = map[string]any{}
		}
		return host.InvokeAction(ctx, args.APIName, args.Input)
	default:
		return nil, fmt.Errorf("unsupported SDK method %q", method)
	}
}
