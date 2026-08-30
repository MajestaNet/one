package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/connectoroauth"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/egress"
	oneotel "github.com/MajestaNet/ide/internal/otel"
	"github.com/MajestaNet/ide/internal/secretcrypt"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// OutboundBridge is implemented by hosts that can perform async HTTPS calls.
type OutboundBridge interface {
	HTTPCall(ctx context.Context, args HTTPCallArgs) (map[string]any, error)
	ConnectorCall(ctx context.Context, args ConnectorCallArgs) (map[string]any, error)
}

// HTTPCallArgs is the guest ctx.http payload.
type HTTPCallArgs struct {
	Method           string            `json:"method"`
	URL              string            `json:"url"`
	ConnectorAPIName string            `json:"connectorApiName"`
	Path             string            `json:"path"`
	Headers          map[string]string `json:"headers"`
	Body             any               `json:"body"`
	SecretRef        string            `json:"secretRef"`
}

// ConnectorCallArgs is the guest ctx.connector payload.
type ConnectorCallArgs struct {
	APIName string            `json:"apiName"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

// SyncOutboundBan rejects outbound methods (used when SyncMode=true).
type SyncOutboundBan struct {
	Inner HostBridge
}

func (s SyncOutboundBan) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return s.Inner.CreateRecord(ctx, objectAPIName, data)
}
func (s SyncOutboundBan) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return s.Inner.UpdateRecord(ctx, objectAPIName, recordID, data)
}
func (s SyncOutboundBan) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return s.Inner.GetRecord(ctx, objectAPIName, recordID)
}
func (s SyncOutboundBan) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	return s.Inner.DeleteRecord(ctx, objectAPIName, recordID)
}
func (s SyncOutboundBan) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	return s.Inner.Query(ctx, req)
}
func (s SyncOutboundBan) InvokeAction(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
	return s.Inner.InvokeAction(ctx, apiName, input)
}
func (s SyncOutboundBan) HandleUnitRPC(ctx context.Context, method string, argsJSON json.RawMessage) (any, bool, error) {
	return forwardUnitRPC(s.Inner, ctx, method, argsJSON)
}
func (s SyncOutboundBan) HTTPCall(context.Context, HTTPCallArgs) (map[string]any, error) {
	return nil, fmt.Errorf("http is not allowed in sync automations")
}
func (s SyncOutboundBan) ConnectorCall(context.Context, ConnectorCallArgs) (map[string]any, error) {
	return nil, fmt.Errorf("connector is not allowed in sync automations")
}

// OutboundHost performs allowlisted HTTPS via the Go host (Deno stays deny-net).
type OutboundHost struct {
	Inner          HostBridge
	Pool           *db.Pool
	EncryptionKey  string
	HTTPClient     *http.Client
	AllowlistCache []string // optional preloaded; reloads from DB when nil
	// ValidateFunc overrides SSRF checks (tests only); nil uses egress.ValidateURL.
	ValidateFunc func(string) error
}

func (h OutboundHost) CreateRecord(ctx context.Context, objectAPIName string, data map[string]any) (string, error) {
	return h.Inner.CreateRecord(ctx, objectAPIName, data)
}
func (h OutboundHost) UpdateRecord(ctx context.Context, objectAPIName, recordID string, data map[string]any) error {
	return h.Inner.UpdateRecord(ctx, objectAPIName, recordID, data)
}
func (h OutboundHost) GetRecord(ctx context.Context, objectAPIName, recordID string) (map[string]any, error) {
	return h.Inner.GetRecord(ctx, objectAPIName, recordID)
}
func (h OutboundHost) DeleteRecord(ctx context.Context, objectAPIName, recordID string) error {
	return h.Inner.DeleteRecord(ctx, objectAPIName, recordID)
}
func (h OutboundHost) Query(ctx context.Context, req map[string]any) (map[string]any, error) {
	return h.Inner.Query(ctx, req)
}
func (h OutboundHost) InvokeAction(ctx context.Context, apiName string, input map[string]any) (map[string]any, error) {
	return h.Inner.InvokeAction(ctx, apiName, input)
}

func (h OutboundHost) HandleUnitRPC(ctx context.Context, method string, argsJSON json.RawMessage) (any, bool, error) {
	return forwardUnitRPC(h.Inner, ctx, method, argsJSON)
}

func (h OutboundHost) HTTPCall(ctx context.Context, args HTTPCallArgs) (map[string]any, error) {
	if strings.TrimSpace(args.ConnectorAPIName) != "" {
		return h.ConnectorCall(ctx, ConnectorCallArgs{
			APIName: args.ConnectorAPIName,
			Method:  args.Method,
			Path:    args.Path,
			Headers: args.Headers,
			Body:    args.Body,
		})
	}
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPost {
		return nil, fmt.Errorf("method %q not allowed (GET|POST only)", method)
	}
	target := strings.TrimSpace(args.URL)
	if target == "" {
		return nil, fmt.Errorf("http requires url or connectorApiName")
	}
	headers := args.Headers
	if args.SecretRef != "" {
		token, err := h.resolveSecret(ctx, args.SecretRef)
		if err != nil {
			return nil, err
		}
		if headers == nil {
			headers = map[string]string{}
		}
		if _, ok := headers["Authorization"]; !ok {
			headers["Authorization"] = "Bearer " + token
		}
	}
	return h.doRequest(ctx, method, target, "", headers, args.Body)
}

func (h OutboundHost) ConnectorCall(ctx context.Context, args ConnectorCallArgs) (map[string]any, error) {
	if h.Pool == nil {
		return nil, fmt.Errorf("connector host not configured")
	}
	apiName := strings.TrimSpace(args.APIName)
	if apiName == "" {
		return nil, fmt.Errorf("connector requires apiName")
	}
	conn, err := db.GetInstallConnector(ctx, h.Pool, apiName)
	if err != nil {
		return nil, fmt.Errorf("connector %q: %w", apiName, err)
	}
	if !conn.Active {
		return nil, fmt.Errorf("connector %q is inactive", apiName)
	}
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		method = http.MethodGet
	}
	allowed := false
	for _, m := range conn.AllowedMethods {
		if strings.EqualFold(m, method) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("method %q not allowed on connector %q", method, apiName)
	}
	path := args.Path
	if path == "" {
		path = conn.PathPrefix
	} else if conn.PathPrefix != "" && !strings.HasPrefix(path, conn.PathPrefix) {
		// When path_prefix set, require path under prefix (or empty → use prefix).
		if !strings.HasPrefix(strings.TrimPrefix(path, "/"), strings.TrimPrefix(conn.PathPrefix, "/")) {
			path = strings.TrimRight(conn.PathPrefix, "/") + "/" + strings.TrimLeft(path, "/")
		}
	}
	target, err := egress.JoinURL(conn.BaseURL, path)
	if err != nil {
		return nil, err
	}
	headers := cloneHeaders(args.Headers)
	stripAuthorization(headers)
	authType := connectoroauth.NormalizeAuthType(conn.AuthType)
	switch authType {
	case connectoroauth.AuthStaticBearer:
		if conn.SecretRef != nil && *conn.SecretRef != "" {
			token, err := h.resolveSecret(ctx, *conn.SecretRef)
			if err != nil {
				return nil, err
			}
			headers["Authorization"] = "Bearer " + token
		}
	case connectoroauth.AuthOAuth2ClientCredentials, connectoroauth.AuthOAuth2AuthorizationCode:
		tok, err := h.resolveOAuthAccessToken(ctx, conn)
		if err != nil {
			return nil, err
		}
		typ := tok.TokenType
		if typ == "" {
			typ = "Bearer"
		}
		headers["Authorization"] = typ + " " + tok.AccessToken
	default:
		return nil, fmt.Errorf("unsupported connector authType %q", authType)
	}
	return h.doRequest(ctx, method, target, apiName, headers, args.Body)
}

func cloneHeaders(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stripAuthorization(headers map[string]string) {
	for k := range headers {
		if strings.EqualFold(k, "Authorization") {
			delete(headers, k)
		}
	}
}

func (h OutboundHost) resolveOAuthAccessToken(ctx context.Context, conn *db.InstallConnector) (*connectoroauth.StoredToken, error) {
	authType := connectoroauth.NormalizeAuthType(conn.AuthType)
	if err := connectoroauth.ValidateFlow(authType, conn.OAuthFlow); err != nil {
		return nil, err
	}
	allow, err := h.allowlist(ctx)
	if err != nil {
		return nil, err
	}
	if !egress.HostAllowed(mustHost(conn.OAuthFlow.TokenURL), allow) {
		return nil, fmt.Errorf("tokenUrl host not on install egress allowlist")
	}
	clientSecret := ""
	if conn.SecretRef != nil && *conn.SecretRef != "" {
		clientSecret, err = h.resolveSecret(ctx, *conn.SecretRef)
		if err != nil {
			return nil, err
		}
	}
	httpClient := h.HTTPClient
	if httpClient == nil {
		httpClient = egress.Client
	}
	tokClient := &connectoroauth.TokenHTTPClient{HTTPClient: httpClient}

	var stored *connectoroauth.StoredToken
	row, err := db.GetInstallConnectorOAuthToken(ctx, h.Pool, conn.APIName)
	if err == nil && row != nil && row.TokenCiphertext != "" {
		plain, derr := secretcrypt.Decrypt(row.TokenCiphertext, h.EncryptionKey)
		if derr != nil {
			return nil, fmt.Errorf("decrypt oauth token: %w", derr)
		}
		var tok connectoroauth.StoredToken
		if uerr := json.Unmarshal([]byte(plain), &tok); uerr != nil {
			return nil, fmt.Errorf("decode oauth token: %w", uerr)
		}
		stored = &tok
	} else if err != nil {
		// Treat missing token row as not connected; other DB errors fail.
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	if stored != nil && !connectoroauth.NeedsRefresh(stored) {
		return stored, nil
	}

	var fresh *connectoroauth.StoredToken
	switch authType {
	case connectoroauth.AuthOAuth2ClientCredentials:
		fresh, err = tokClient.ClientCredentials(ctx, conn.OAuthFlow, clientSecret)
	case connectoroauth.AuthOAuth2AuthorizationCode:
		if stored == nil || stored.RefreshToken == "" {
			return nil, fmt.Errorf("connector %q requires authorization (POST /auth/v1/connectors/%s/authorize)", conn.APIName, conn.APIName)
		}
		fresh, err = tokClient.RefreshAccessToken(ctx, conn.OAuthFlow, stored.RefreshToken, clientSecret)
	default:
		return nil, fmt.Errorf("unsupported oauth authType")
	}
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(fresh)
	ct, err := secretcrypt.Encrypt(string(raw), h.EncryptionKey)
	if err != nil {
		return nil, err
	}
	var exp *time.Time
	if !fresh.Expiry.IsZero() {
		e := fresh.Expiry.UTC()
		exp = &e
	}
	if err := db.UpsertInstallConnectorOAuthToken(ctx, h.Pool, conn.APIName, ct, exp, fresh.RefreshToken != ""); err != nil {
		return nil, err
	}
	return fresh, nil
}

func mustHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func (h OutboundHost) resolveSecret(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || h.Pool == nil {
		return "", fmt.Errorf("secret ref required")
	}
	ct, err := db.GetInstallSecretCiphertext(ctx, h.Pool, ref)
	if err != nil {
		return "", fmt.Errorf("secret %q: %w", ref, err)
	}
	plain, err := secretcrypt.Decrypt(ct, h.EncryptionKey)
	if err != nil {
		return "", fmt.Errorf("decrypt secret %q: %w", ref, err)
	}
	return plain, nil
}

func (h OutboundHost) allowlist(ctx context.Context) ([]string, error) {
	if h.AllowlistCache != nil {
		return h.AllowlistCache, nil
	}
	if h.Pool == nil {
		return nil, fmt.Errorf("egress allowlist unavailable")
	}
	return db.ListEgressHostPatterns(ctx, h.Pool)
}

func (h OutboundHost) doRequest(ctx context.Context, method, target, connectorAPIName string, headers map[string]string, body any) (map[string]any, error) {
	tr := oneotel.Tracer("one.egress")
	ctx, span := tr.Start(ctx, "egress.http")
	defer span.End()
	span.SetAttributes(
		attribute.String("http.method", method),
		attribute.String("url.full", oneotel.RedactURL(target)),
	)
	if connectorAPIName != "" {
		span.SetAttributes(attribute.String("one.connector", connectorAPIName))
	}

	validate := h.ValidateFunc
	if validate == nil {
		validate = egress.ValidateURL
	}
	if err := validate(target); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ssrf")
		return nil, fmt.Errorf("egress validation: %w", err)
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	allow, err := h.allowlist(ctx)
	if err != nil {
		return nil, err
	}
	if !egress.HostAllowed(u.Hostname(), allow) {
		err := fmt.Errorf("host %q not on install egress allowlist", u.Hostname())
		span.RecordError(err)
		span.SetStatus(codes.Error, "allowlist")
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil && method != http.MethodGet {
		switch b := body.(type) {
		case string:
			bodyReader = strings.NewReader(b)
		case []byte:
			bodyReader = bytes.NewReader(b)
		default:
			raw, err := json.Marshal(b)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewReader(raw)
			if headers == nil {
				headers = map[string]string{}
			}
			if _, ok := headers["Content-Type"]; !ok {
				headers["Content-Type"] = "application/json"
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range egress.FilterHeaders(headers) {
		for _, vv := range v {
			req.Header.Set(k, vv)
		}
	}

	client := h.HTTPClient
	if client == nil {
		client = egress.Client
	}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "transport")
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	limited := io.LimitReader(resp.Body, egress.MaxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(raw) > egress.MaxResponseBytes {
		err := fmt.Errorf("response exceeds %d byte limit", egress.MaxResponseBytes)
		span.RecordError(err)
		span.SetStatus(codes.Error, "body_limit")
		return nil, err
	}

	out := map[string]any{
		"status":  resp.StatusCode,
		"ok":      resp.StatusCode >= 200 && resp.StatusCode < 300,
		"headers": flattenResponseHeaders(resp.Header),
		"body":    string(raw),
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var parsed any
		if json.Unmarshal(raw, &parsed) == nil {
			out["json"] = parsed
		}
	}
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, "http_error")
	}
	return out, nil
}

func flattenResponseHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, vals := range h {
		lk := strings.ToLower(k)
		if lk == "set-cookie" || lk == "authorization" {
			continue
		}
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}
