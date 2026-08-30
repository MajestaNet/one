package httpapi

import (
	"context"
	"net/http"

	"github.com/MajestaNet/ide/internal/compat"
)

type apiRevisionCtxKey struct{}

// APIRevisionFromContext returns the resolved API revision pin for the request.
// Handlers should branch on this only when a real per-revision adapter exists
// (ADR-025 Phase 4). Until then, all in-window pins share identical behavior.
func APIRevisionFromContext(ctx context.Context) (int, bool) {
	v := ctx.Value(apiRevisionCtxKey{})
	if v == nil {
		return 0, false
	}
	pin, ok := v.(int)
	return pin, ok
}

func (s *Server) compatDiscoveryFields() map[string]any {
	out := map[string]any{
		"productVersion": s.cfg.ProductVersion,
		"httpApi":        compat.DefaultHTTPAPI(),
	}
	min, current, err := s.cfg.APIRevisionWindow()
	if err != nil {
		out["apiRevision"] = map[string]any{"min": 1, "current": 1, "recommended": 1}
		return out
	}
	out["apiRevision"] = compat.APIRevisionWindow{Min: min, Current: current}
	return out
}

// applyAPIRevision enforces One-API-Revision (or /r{N}/) on family/MCP paths,
// rewrites the path alias onto the stable family route, and stashes the pin.
func (s *Server) applyAPIRevision(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	if !compat.PathRequiresRevision(r.URL.Path) {
		return r, true
	}
	rewritten, pathPin, pathFound := compat.SplitRevisionPath(r.URL.Path)
	min, current, err := s.cfg.APIRevisionWindow()
	if err != nil {
		writeRevisionUnsupported(w, 0, compat.APIRevisionWindow{Min: 1, Current: 1}, "UNPARSEABLE_REVISION")
		return r, false
	}
	window := compat.APIRevisionWindow{Min: min, Current: current}
	pin, _, err := compat.ResolveRequestPin(r.Header.Get("One-API-Revision"), pathPin, pathFound, window)
	if err != nil {
		writeRevisionUnsupported(w, 0, window, "API_REVISION_UNSUPPORTED")
		return r, false
	}
	if !compat.PinInWindow(pin, window) {
		writeRevisionUnsupported(w, pin, window, "API_REVISION_UNSUPPORTED")
		return r, false
	}
	if rewritten != r.URL.Path {
		r = cloneWithPath(r, rewritten)
	}
	return s.withAPIRevisionContext(r, pin), true
}

func writeRevisionUnsupported(w http.ResponseWriter, pin int, window compat.APIRevisionWindow, code string) {
	message := "API revision is outside the supported window for this install"
	cta := compat.UnsupportedCTA(pin, window)
	if code == "UNPARSEABLE_REVISION" {
		message = "Install API revision window is invalid"
		cta = "Fix API_REVISION_CURRENT / API_REVISION_MIN on the install"
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"error":   code,
		"message": message,
		"pin":     pin,
		"min":     window.Min,
		"current": window.Current,
		"cta":     cta,
	})
}

func (s *Server) withAPIRevisionContext(r *http.Request, pin int) *http.Request {
	ctx := context.WithValue(r.Context(), apiRevisionCtxKey{}, pin)
	return r.WithContext(ctx)
}

func cloneWithPath(r *http.Request, newPath string) *http.Request {
	nr := r.Clone(r.Context())
	u := *r.URL
	u.Path = newPath
	u.RawPath = ""
	nr.URL = &u
	if r.RequestURI != "" {
		if u.RawQuery != "" {
			nr.RequestURI = newPath + "?" + u.RawQuery
		} else {
			nr.RequestURI = newPath
		}
	}
	return nr
}
