package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MajestaNet/ide/internal/actions"
	"github.com/MajestaNet/ide/internal/authlogin"
	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/edge"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/ops"
	"github.com/MajestaNet/ide/internal/version"
)

type ctxKey int

const actorKey ctxKey = 1

// Pinger is optional DB readiness (pgx pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server is the Majesta One HTTP API (Go).
type Server struct {
	cfg              *config.Config
	resolver         *authz.Resolver
	db               Pinger
	pool             *db.Pool
	meta             *metadata.Service
	data             *dataengine.Service
	deploy           *deploy.DeployEngine
	ops              *ops.Engine
	objectAz         *authz.ObjectAuthz
	fieldAz          *authz.FieldAuthz
	systemAz         *authz.SystemAuthz
	automationAz     *authz.AutomationAuthz
	toolAz           *authz.ToolAuthz
	recordAccess     *authz.RecordAccessEvaluator
	actions          *actions.Service
	edgeRoller       edge.Roller
	identity         identity.Backend
	loginBroker      *authlogin.Broker
	mux              *http.ServeMux
	authTokenLimiter *rateLimiter
	ready            atomic.Bool
}

// Options configures the HTTP server.
type Options struct {
	Config       *config.Config
	Resolver     *authz.Resolver
	DB           Pinger
	Pool         *db.Pool
	Metadata     *metadata.Service
	DataEngine   *dataengine.Service
	Deploy       *deploy.DeployEngine
	Ops          *ops.Engine
	ObjectAz     *authz.ObjectAuthz
	FieldAz      *authz.FieldAuthz
	SystemAz     *authz.SystemAuthz
	AutomationAz *authz.AutomationAuthz
	ToolAz       *authz.ToolAuthz
	RecordAccess *authz.RecordAccessEvaluator
	Actions      *actions.Service
	EdgeRoller   edge.Roller
	Identity     identity.Backend
	LoginBroker  *authlogin.Broker
}

// New constructs the API server.
func New(opts Options) *Server {
	s := &Server{
		cfg:          opts.Config,
		resolver:     opts.Resolver,
		db:           opts.DB,
		pool:         opts.Pool,
		meta:         opts.Metadata,
		data:         opts.DataEngine,
		deploy:       opts.Deploy,
		ops:          opts.Ops,
		objectAz:     opts.ObjectAz,
		fieldAz:      opts.FieldAz,
		systemAz:     opts.SystemAz,
		automationAz: opts.AutomationAz,
		toolAz:       opts.ToolAz,
		recordAccess: opts.RecordAccess,
		actions:      opts.Actions,
		edgeRoller:   opts.EdgeRoller,
		identity:     opts.Identity,
		loginBroker:  opts.LoginBroker,
		mux:          http.NewServeMux(),
	}
	if opts.Config != nil {
		s.authTokenLimiter = newRateLimiter(opts.Config.AuthTokenRateLimitPerMinute)
	}
	if s.identity == nil {
		s.identity = identity.NopBackend{}
	}
	if s.loginBroker == nil && opts.Config != nil {
		s.loginBroker = authlogin.NewBroker(
			authlogin.GoogleConfig{ClientID: opts.Config.AuthGoogleClientID, ClientSecret: opts.Config.AuthGoogleClientSecret},
			authlogin.AppleConfig{
				ClientID: opts.Config.AuthAppleClientID, TeamID: opts.Config.AuthAppleTeamID,
				KeyID: opts.Config.AuthAppleKeyID, PrivateKey: opts.Config.AuthApplePrivateKey,
			},
			opts.Config.LoginProviderEnabled(identity.ProviderDev),
		)
		if slack := authlogin.NewSlackProvider(authlogin.SlackConfig{
			ClientID:     opts.Config.AuthSlackClientID,
			ClientSecret: opts.Config.AuthSlackClientSecret,
		}); slack.Configured() {
			s.loginBroker.RegisterOrReplace(slack)
		}
	}
	if s.actions == nil {
		s.actions = actions.New(actions.Options{
			Meta:         s.meta,
			Data:         s.data,
			ObjectAz:     s.objectAz,
			FieldAz:      s.fieldAz,
			RecordAccess: s.recordAccess,
		})
	}
	if s.data != nil && s.data.Actions == nil {
		s.data.Actions = s.actions
	}
	s.routes()
	s.ready.Store(true)
	return s
}

// SetReady toggles whether non-probe routes are served.
// cmd/api sets this false until bootstrap/seed finishes so /healthz is
// reachable on localhost while seed still runs.
func (s *Server) SetReady(ready bool) {
	if s == nil {
		return
	}
	s.ready.Store(ready)
}

// Ready reports whether bootstrap/seed has completed.
func (s *Server) Ready() bool {
	return s != nil && s.ready.Load()
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("GET /version", s.handleVersion)

	// Auth Token Service (ADR-006 / BP-013) — unauthenticated mint/exchange
	s.registerAuthRoutes()

	// SCIM 2.0 identity adapter (Client identity SoR)
	s.registerSCIMRoutes()

	// Canonical Client
	s.registerClientFamily("/client/v1")
	// Canonical Metadata
	s.mux.Handle("GET /metadata/v1/objects", s.requireAuth(s.requireScope(authz.ScopeMetadata, http.HandlerFunc(s.handleListObjects))))
	s.registerMetadataWrites("/metadata/v1")
	s.registerSharingRoutes("/metadata/v1")
	s.registerMetadataCoreAliases("/metadata/v1/metadata")
	// Deploy
	s.registerDeployRoutes()
	s.registerDeployCloudRoutes()
	// Ops (product upgrades — ADR-007)
	s.registerOpsRoutes()
	// Flat /v1 aliases (Client + Metadata; Deploy/Ops have no flat alias)
	s.registerClientFamily("/v1")
	s.mux.Handle("GET /v1/objects", s.requireAuth(s.requireScope(authz.ScopeMetadata, http.HandlerFunc(s.handleListObjects))))
	s.registerMetadataWrites("/v1")
	s.registerSharingRoutes("/v1")
	s.registerMetadataCoreAliases("/v1/metadata")
	// MCP adapter (ADR-010) — gated by FEATURE_FLAGS agents
	s.registerMCPRoutes()
}

func (s *Server) registerClientFamily(prefix string) {
	client := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, h))
	}
	s.mux.Handle("GET "+prefix+"/me", client(s.handleMe))
	s.mux.Handle("POST "+prefix+"/me/password", client(s.handleChangeMyPassword))
	s.mux.Handle("GET "+prefix+"/describe", client(s.handleDescribeGlobal))
	s.mux.Handle("GET "+prefix+"/describe/{object}", client(s.handleDescribeObject))
	s.mux.Handle("POST "+prefix+"/sobjects/{object}", client(s.handleCreateSObject))
	s.mux.Handle("POST "+prefix+"/sobjects/{object}/upsert", client(s.handleUpsertSObject))
	s.mux.Handle("GET "+prefix+"/sobjects/{object}/{externalIdField}/{externalId}", client(s.handleGetSObjectByExternalID))
	s.mux.Handle("PATCH "+prefix+"/sobjects/{object}/{externalIdField}/{externalId}", client(s.handleUpsertSObjectByExternalID))
	s.mux.Handle("DELETE "+prefix+"/sobjects/{object}/{externalIdField}/{externalId}", client(s.handleDeleteSObjectByExternalID))
	s.mux.Handle("GET "+prefix+"/sobjects/{object}/{id}", client(s.handleGetSObject))
	s.mux.Handle("PATCH "+prefix+"/sobjects/{object}/{id}", client(s.handlePatchSObject))
	s.mux.Handle("DELETE "+prefix+"/sobjects/{object}/{id}", client(s.handleDeleteSObject))
	s.mux.Handle("POST "+prefix+"/query", client(s.handleQuery))
	s.mux.Handle("POST "+prefix+"/search", client(s.handleSearch))
	s.mux.Handle("POST "+prefix+"/composite", client(s.handleComposite))
	s.mux.Handle("POST "+prefix+"/bulk/{object}", client(s.handleBulk))
	s.registerIngestRoutes(prefix, client)
	s.registerClientExtras(prefix)
	s.registerDataRoleRoutes(prefix)
	s.registerDeviceRoutes(prefix)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "one-api", "runtime": "go"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "starting", "database": "migrated"})
		return
	}
	if !pingerConfigured(s.db) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "database": "skipped"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func pingerConfigured(p Pinger) bool {
	if p == nil {
		return false
	}
	if pool, ok := p.(*db.Pool); ok {
		return pool != nil
	}
	return true
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	body := map[string]any{
		"version":        version.Version,
		"productVersion": s.cfg.ProductVersion,
		"runtime":        "go",
	}
	for k, v := range s.compatDiscoveryFields() {
		body[k] = v
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	actor := ActorFromContext(r.Context())
	if actor != nil && s.systemAz != nil {
		if caps, err := s.systemAz.EffectiveCapabilities(r.Context(), actor); err == nil {
			actor.SystemPermissions = caps
		}
	}
	out := map[string]any{}
	if actor != nil {
		b, err := json.Marshal(actor)
		if err == nil {
			_ = json.Unmarshal(b, &out)
		}
	}
	for k, v := range s.compatDiscoveryFields() {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleChangeMyPassword(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	creds := db.NewCredentialStore(pool)
	ok, err := creds.VerifyPassword(r.Context(), actor.ID, body.CurrentPassword)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusUnauthorized, "INVALID_PASSWORD", "current password rejected")
		return
	}
	if _, err := creds.SetPassword(r.Context(), actor.ID, body.NewPassword); err != nil {
		if errors.Is(err, db.ErrValidation) {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeAPIError(w, err)
		return
	}
	if err := s.revokeRefreshForUser(r.Context(), actor.ID); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "principal.password.set", "", nil, map[string]any{
		"userId": actor.ID, "source": "self",
	})
	_ = db.EnqueueOutbox(r.Context(), pool, db.EventPrincipalPasswordChanged, actor.ID, map[string]any{
		"userId": actor.ID, "actorId": actor.ID, "source": "self",
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDescribeGlobal(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	desc, err := s.meta.DescribeGlobal(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	actor := ActorFromContext(r.Context())
	filtered, err := filterDescribeGlobal(r.Context(), s.objectAz, actor, desc)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleDescribeObject(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	object := r.PathValue("object")
	desc, err := s.meta.Describe(r.Context(), object)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	actor := ActorFromContext(r.Context())
	filtered, err := filterDescribeObject(r.Context(), s.objectAz, s.fieldAz, actor, desc)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	if s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "metadata service not configured")
		return
	}
	objs, err := s.meta.ListObjects(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"objects": objs})
}

func (s *Server) handleCreateSObject(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, object, authz.ActionCreate); err != nil {
		writeAPIError(w, err)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	if err := s.assertOwnerIDWrite(r.Context(), actor, object, body); err != nil {
		writeAPIError(w, err)
		return
	}
	if s.fieldAz != nil {
		if err := s.fieldAz.AssertEditableFields(r.Context(), actor, object, body); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	rec, err := s.data.Create(r.Context(), object, body, actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if s.fieldAz != nil {
		rec, err = s.fieldAz.StripUnreadableFields(r.Context(), actor, object, rec)
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleGetSObject(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	id := r.PathValue("id")
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, object, authz.ActionRead); err != nil {
		writeAPIError(w, err)
		return
	}
	rec, err := s.data.Get(r.Context(), object, id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	viewAll, err := s.objectAz.GetViewAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ownerID, _ := rec["OwnerId"].(string)
	createdByID, _ := rec["CreatedById"].(string)
	idVal, _ := rec["Id"].(string)
	ok, err := s.canViewRecord(r.Context(), actor, idVal, ownerID, createdByID, object, viewAll)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Not allowed")
		return
	}
	if s.fieldAz != nil {
		rec, err = s.fieldAz.StripUnreadableFields(r.Context(), actor, object, rec)
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handlePatchSObject(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	id := r.PathValue("id")
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, object, authz.ActionUpdate); err != nil {
		writeAPIError(w, err)
		return
	}
	existing, err := s.data.Get(r.Context(), object, id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	modifyAll, err := s.objectAz.GetModifyAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ownerID, _ := existing["OwnerId"].(string)
	createdByID, _ := existing["CreatedById"].(string)
	ok, err := s.canModifyRecord(r.Context(), actor, id, ownerID, createdByID, object, modifyAll)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Not allowed")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	if err := s.assertOwnerIDWrite(r.Context(), actor, object, body); err != nil {
		writeAPIError(w, err)
		return
	}
	if s.fieldAz != nil {
		if err := s.fieldAz.AssertEditableFields(r.Context(), actor, object, body); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	rec, err := s.data.Update(r.Context(), object, id, body, actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if s.fieldAz != nil {
		rec, err = s.fieldAz.StripUnreadableFields(r.Context(), actor, object, rec)
		if err != nil {
			writeAPIError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleDeleteSObject(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	id := r.PathValue("id")
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, object, authz.ActionDelete); err != nil {
		writeAPIError(w, err)
		return
	}
	existing, err := s.data.Get(r.Context(), object, id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	modifyAll, err := s.objectAz.GetModifyAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ownerID, _ := existing["OwnerId"].(string)
	createdByID, _ := existing["CreatedById"].(string)
	ok, err := s.canModifyRecord(r.Context(), actor, id, ownerID, createdByID, object, modifyAll)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Not allowed")
		return
	}
	if err := s.data.Delete(r.Context(), object, id, actor); err != nil {
		writeAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// assertOwnerIDWrite enforces OwnerId assignment policy when the field is present in the body.
func (s *Server) assertOwnerIDWrite(ctx context.Context, actor *authz.Actor, object string, body map[string]any) error {
	v, ok := body["OwnerId"]
	if !ok {
		return nil
	}
	modifyAll, err := s.objectAz.GetModifyAllObjects(ctx, actor)
	if err != nil {
		return err
	}
	var newOwner *string
	if v != nil {
		if s, ok := v.(string); ok && s != "" {
			newOwner = &s
		}
	}
	return authz.AssertOwnerIDWritable(actor, object, newOwner, modifyAll)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	raw, err := readBodyLimited(r.Body, 1<<20)
	if err != nil {
		if requestBodyTooLarge(err) {
			writeErr(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unable to read body")
		return
	}
	var peek struct {
		Object string `json:"object"`
	}
	_ = json.Unmarshal(raw, &peek)
	if peek.Object == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "object is required")
		return
	}
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, peek.Object, authz.ActionRead); err != nil {
		writeAPIError(w, err)
		return
	}
	viewAll, err := s.objectAz.GetViewAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	vis, err := s.buildQueryVisibility(r.Context(), actor, peek.Object, viewAll)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	result, err := s.data.Query(r.Context(), raw, vis)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	// Visibility is enforced in SQL via QueryVisibility; do not post-filter rows
	// (that under-fills pages and skews nextCursor). Retain FLS strip only.
	records := result.Records
	if s.fieldAz != nil {
		flsCtx := authz.ContextWithFLSCache(r.Context())
		stripped := make([]dataengine.SObjectRecord, 0, len(records))
		for _, rec := range records {
			outRec, err := s.fieldAz.StripUnreadableFields(flsCtx, actor, peek.Object, rec)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			stripped = append(stripped, outRec)
		}
		records = stripped
	}
	out := map[string]any{
		"records":   records,
		"totalSize": len(records),
		"done":      result.Done,
		"queryPlan": result.QueryPlan,
	}
	if result.NextCursor != "" {
		out["nextCursor"] = result.NextCursor
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="one"`)
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing bearer token or API key")
			return
		}
		if s.resolver == nil {
			writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "authentication service not configured")
			return
		}
		actor, err := s.resolver.ResolveBearer(token)
		if err != nil {
			if errors.Is(err, authz.ErrPrincipalNoRole) {
				writeErr(w, http.StatusForbidden, "FORBIDDEN", "Principal has no assigned role")
				return
			}
			w.Header().Set("WWW-Authenticate", `Bearer realm="one"`)
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials")
			return
		}
		policy, err := s.loadExposurePolicy(r.Context())
		if err != nil {
			writeErr(w, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "access policy unavailable")
			return
		}
		if err := s.enforceClientAccessMode(actor, policy); err != nil {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if err := s.enforceDeviceCertIfRequired(r, actor, policy); err != nil {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), actorKey, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) loadExposurePolicy(ctx context.Context) (edge.Policy, error) {
	if s.pool == nil {
		return edge.DefaultPolicy(), nil
	}
	store := db.NewExposureStore(s.pool)
	row, err := store.Get(ctx)
	if err != nil {
		return edge.Policy{}, err
	}
	p := row.Policy
	if p.ClientAccessMode == edge.ClientAccessIDEUsers {
		p.ClientAccessMode = p.EffectiveClientAccessMode()
	}
	if err := edge.Validate(p); err != nil {
		return edge.Policy{}, fmt.Errorf("invalid stored exposure policy: %w", err)
	}
	return p, nil
}

func (s *Server) enforceClientAccessMode(actor *authz.Actor, p edge.Policy) error {
	if actor == nil {
		return nil
	}
	mode := p.EffectiveClientAccessMode()
	if mode == edge.ClientAccessOpen {
		return nil
	}
	isAPIKey := actor.AuthMethod == "api_key"
	return authz.AllowBearerAzp(mode, actor.Azp, actor.AuthMethod, isAPIKey)
}

func (s *Server) enforceDeviceCertIfRequired(r *http.Request, actor *authz.Actor, p edge.Policy) error {
	if actor == nil || actor.AuthMethod == "api_key" {
		return nil
	}
	if !p.RequireDeviceCert {
		return nil
	}
	// Enrollment and revoke must be reachable without an existing device header.
	if deviceCertBootstrapPath(r.Method, r.URL.Path) {
		return nil
	}
	deviceID := strings.TrimSpace(r.Header.Get("X-One-Device-Id"))
	if deviceID == "" {
		return fmt.Errorf("%w: X-One-Device-Id required when requireDeviceCert=true", authz.ErrForbidden)
	}
	if s.pool == nil {
		return nil
	}
	ok, err := db.DeviceCertActive(r.Context(), s.pool, actor.ID, deviceID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: device certificate not enrolled or revoked", authz.ErrForbidden)
	}
	return nil
}

func deviceCertBootstrapPath(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	for _, prefix := range []string{"/client/v1/devices/", "/v1/devices/"} {
		if path == prefix+"enroll" {
			return true
		}
		if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, "/revoke") {
			deviceID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/revoke")
			return deviceID != "" && !strings.Contains(deviceID, "/")
		}
	}
	return false
}

func (s *Server) requireScope(scope authz.Scope, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := ActorFromContext(r.Context())
		if actor == nil || !actor.HasScope(scope) {
			have := "none"
			if actor != nil {
				parts := make([]string, len(actor.Scopes))
				for i, sc := range actor.Scopes {
					parts[i] = string(sc)
				}
				have = strings.Join(parts, ", ")
			}
			writeErr(w, http.StatusForbidden, "FORBIDDEN",
				`Credential lacks required scope "`+string(scope)+`" (have: `+have+`)`)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ActorFromContext returns the authenticated actor.
func ActorFromContext(ctx context.Context) *authz.Actor {
	a, _ := ctx.Value(actorKey).(*authz.Actor)
	return a
}

func bearerToken(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get("Authorization")); h != "" {
		parts := strings.Fields(h)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return ""
		}
		return parts[1]
	}
	return strings.TrimSpace(r.Header.Get("x-api-key"))
}

func readBodyLimited(body io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, &http.MaxBytesError{Limit: limit}
	}
	return raw, nil
}

func requestBodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeErrDetails(w, status, code, message, nil)
}

func writeErrDetails(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	body := map[string]any{
		"error":   code,
		"message": message,
	}
	if len(details) > 0 {
		body["details"] = details
	}
	writeJSON(w, status, body)
}

func writeAPIError(w http.ResponseWriter, err error) {
	var ae *actions.Error
	if errors.As(err, &ae) {
		writeErrDetails(w, ae.Status, ae.Code, ae.Message, ae.Details)
		return
	}
	var ve *dataengine.ValidationError
	var dve *deploy.ValidationError
	switch {
	case errors.As(err, &ve):
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", ve.Error())
	case errors.As(err, &dve):
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", dve.Error())
	case errors.Is(err, dataengine.ErrConflict) && strings.Contains(err.Error(), "DUPLICATE_EXTERNAL_ID"):
		writeErr(w, http.StatusConflict, "DUPLICATE_EXTERNAL_ID", err.Error())
	case errors.Is(err, metadata.ErrConflict), errors.Is(err, db.ErrConflict), errors.Is(err, dataengine.ErrConflict):
		writeErr(w, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, deploy.ErrBusy):
		var be *deploy.BusyError
		msg := err.Error()
		if errors.As(err, &be) && be != nil {
			msg = be.Error()
		}
		writeErr(w, http.StatusTooManyRequests, "DEPLOY_BUSY", msg)
	case errors.Is(err, deploy.ErrRepoAlreadyInitialized):
		writeErr(w, http.StatusConflict, "REPO_ALREADY_INITIALIZED", "Customer repository already initialized; pass force=true to overwrite main")
	case errors.Is(err, deploy.ErrCustomerRepoNotConfigured):
		writeErr(w, http.StatusServiceUnavailable, "CUSTOMER_REPO_NOT_CONFIGURED", "CUSTOMER_REPO_URL is not configured on this install")
	case errors.Is(err, metadata.ErrNotFound), errors.Is(err, dataengine.ErrNotFound), errors.Is(err, deploy.ErrNotFound), errors.Is(err, db.ErrNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, db.ErrValidation), errors.Is(err, metadata.ErrValidation):
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, db.ErrPrincipalRequiresRole), errors.Is(err, db.ErrLastIdentityAdmin), errors.Is(err, db.ErrPrincipalFrozen):
		writeErr(w, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, authz.ErrPrincipalNoRole), errors.Is(err, deploy.ErrForbidden), errors.Is(err, metadata.ErrForbidden):
		writeErr(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	default:
		var oe *ops.APIError
		if errors.As(err, &oe) {
			writeErr(w, oe.Status, oe.Code, oe.Message)
			return
		}
		slog.Error("internal error", "err", err)
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Internal server error")
	}
}

func randomRequestID() string {
	return fmtRequestID(time.Now().UnixNano())
}

func fmtRequestID(n int64) string {
	return strings.ReplaceAll(time.Unix(0, n).UTC().Format("20060102150405.000000000"), ".", "")
}
