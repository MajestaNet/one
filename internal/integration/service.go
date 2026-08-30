// Package integration implements Connected App / inbound OAuth client configurations.
package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/secretcrypt"
)

const (
	APINameControlIDE  = "one.controlIde"
	OwnershipManaged   = "managed"
	OwnershipCustom    = "custom"
	ClientPublic       = "public"
	ClientConfidential = "confidential"

	// DefaultControlIDERedirectURI is the Electron deep-link callback.
	DefaultControlIDERedirectURI = "one-control://oauth/callback"
)

// Service manages integration configs, linked service principals, and identity write-through.
type Service struct {
	Pool           *db.Pool
	Identity       identity.Backend
	EncryptionKey  string
	IdentityIssuer string // OIDC issuer recorded on identity_links when an adapter is enabled
}

func (s *Service) store() *db.IntegrationStore {
	return db.NewIntegrationStore(s.Pool)
}

func (s *Service) users() *db.UserStore {
	return db.NewUserStore(s.Pool)
}

func (s *Service) creds() *db.CredentialStore {
	return db.NewCredentialStore(s.Pool)
}

func (s *Service) links() *db.IdentityLinkStore {
	return db.NewIdentityLinkStore(s.Pool)
}

// View is the API-safe representation (no secret plaintext).
type View struct {
	ID                string   `json:"id"`
	APIName           string   `json:"apiName"`
	Label             string   `json:"label"`
	Description       string   `json:"description"`
	PrincipalID       string   `json:"principalId"`
	ClientKind        string   `json:"clientKind"`
	OAuthFlows        []string `json:"oauthFlows"`
	CallbackURLs      []string `json:"callbackUrls"`
	LogoutURLs        []string `json:"logoutUrls"`
	AllowedScopesHint []string `json:"allowedScopesHint"`
	AllowedCIDRs      []string `json:"allowedCidrs"`
	PKCERequired      bool     `json:"pkceRequired"`
	Ownership         string   `json:"ownership"`
	PackageName       *string  `json:"packageName,omitempty"`
	IsActive          bool     `json:"isActive"`
	HasOneSecret      bool     `json:"hasOneSecret"`
	CreatedAt         any      `json:"createdAt"`
	UpdatedAt         any      `json:"updatedAt"`
}

func toView(c *db.IntegrationConfig) View {
	return View{
		ID:                c.ID,
		APIName:           c.APIName,
		Label:             c.Label,
		Description:       c.Description,
		PrincipalID:       c.PrincipalID,
		ClientKind:        c.ClientKind,
		OAuthFlows:        append([]string{}, c.OAuthFlows...),
		CallbackURLs:      append([]string{}, c.CallbackURLs...),
		LogoutURLs:        append([]string{}, c.LogoutURLs...),
		AllowedScopesHint: append([]string{}, c.AllowedScopesHint...),
		AllowedCIDRs:      append([]string{}, c.AllowedCIDRs...),
		PKCERequired:      c.PKCERequired,
		Ownership:         c.Ownership,
		PackageName:       c.PackageName,
		IsActive:          c.IsActive,
		HasOneSecret:      c.OneSecretEnc != nil && *c.OneSecretEnc != "",
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

// CreateInput is the customer create payload.
type CreateInput struct {
	APIName           string
	Label             string
	Description       string
	ClientKind        string
	OAuthFlows        []string
	CallbackURLs      []string
	LogoutURLs        []string
	AllowedScopesHint []string
	PKCERequired      *bool
	RoleAPINames      []string
	PrincipalEmail    string
	PrincipalName     string
}

// CreateResult includes the view plus one-time Majesta One secret.
type CreateResult struct {
	View            View
	OneClientSecret string `json:"oneClientSecret,omitempty"`
}

var (
	ErrValidation = errors.New("validation")
	ErrConflict   = errors.New("conflict")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
)

func normalizeFlows(flows []string) ([]string, error) {
	if len(flows) == 0 {
		return nil, fmt.Errorf("%w: oauthFlows required", ErrValidation)
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range flows {
		f = strings.TrimSpace(f)
		switch f {
		case identity.FlowAuthorizationCode, identity.FlowClientCredentials:
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		default:
			return nil, fmt.Errorf("%w: unsupported oauth flow %q", ErrValidation, f)
		}
	}
	return out, nil
}

func validateCreate(in *CreateInput) error {
	in.APIName = strings.TrimSpace(in.APIName)
	if in.APIName == "" {
		return fmt.Errorf("%w: apiName is required", ErrValidation)
	}
	if strings.HasPrefix(in.APIName, "one.") {
		return fmt.Errorf("%w: apiName prefix one. is reserved for managed integrations", ErrValidation)
	}
	kind := strings.TrimSpace(in.ClientKind)
	if kind == "" {
		kind = ClientConfidential
	}
	if kind != ClientPublic && kind != ClientConfidential {
		return fmt.Errorf("%w: clientKind must be public or confidential", ErrValidation)
	}
	in.ClientKind = kind
	flows, err := normalizeFlows(in.OAuthFlows)
	if err != nil {
		return err
	}
	in.OAuthFlows = flows
	if kind == ClientPublic {
		hasCode := false
		for _, f := range flows {
			if f == identity.FlowAuthorizationCode {
				hasCode = true
			}
			if f == identity.FlowClientCredentials {
				return fmt.Errorf("%w: public clients cannot use client_credentials", ErrValidation)
			}
		}
		if !hasCode {
			return fmt.Errorf("%w: public clients require authorization_code", ErrValidation)
		}
	}
	return nil
}

func (s *Service) appSpec(apiName, clientKind string, flows, callbacks, logouts []string) identity.AppClientSpec {
	confidential := clientKind == ClientConfidential
	return identity.AppClientSpec{
		Name:           apiName,
		PrincipalType:  "service",
		Confidential:   confidential,
		OAuthFlows:     flows,
		CallbackURLs:   callbacks,
		LogoutURLs:     logouts,
		GenerateSecret: confidential,
	}
}

// identityAppClientID returns the IdP app client id stored as identity_links.subject for this principal.
func (s *Service) identityAppClientID(ctx context.Context, principalID string) (string, error) {
	if s.Identity == nil || !s.Identity.Enabled() {
		return "", nil
	}
	links, err := s.links().ListByUserID(ctx, principalID)
	if err != nil {
		return "", err
	}
	provider := identity.ProviderForBackend(s.Identity.Mode())
	for _, l := range links {
		if l.Provider == provider && strings.TrimSpace(l.Subject) != "" {
			return strings.TrimSpace(l.Subject), nil
		}
	}
	// Fallback: any non-empty subject on the principal (single integration link).
	for _, l := range links {
		if sub := strings.TrimSpace(l.Subject); sub != "" {
			return sub, nil
		}
	}
	return "", nil
}

// syncIdentity creates or updates an identity app client and stores the link in identity_links only.
func (s *Service) syncIdentity(ctx context.Context, principalID, apiName, clientKind string, flows, callbacks, logouts []string, existingClientID *string) (clientID string, err error) {
	if s.Identity == nil || !s.Identity.Enabled() {
		return "", nil
	}
	spec := s.appSpec(apiName, clientKind, flows, callbacks, logouts)
	if existingClientID != nil && *existingClientID != "" {
		if err := s.Identity.UpdateAppClient(ctx, *existingClientID, spec); err != nil {
			return "", err
		}
		return *existingClientID, nil
	}
	id, _, err := s.Identity.CreateAppClient(ctx, spec)
	if err != nil {
		return "", err
	}
	issuer := strings.TrimSpace(s.IdentityIssuer)
	if _, err := s.links().Upsert(ctx, principalID, identity.ProviderForBackend(s.Identity.Mode()), issuer, id); err != nil {
		return "", err
	}
	return id, nil
}

// Create creates a customer integration + service principal + optional Majesta One secret + identity client link.
func (s *Service) Create(ctx context.Context, in CreateInput) (*CreateResult, error) {
	if err := validateCreate(&in); err != nil {
		return nil, err
	}
	if err := applyPublicClientDefaults(&in); err != nil {
		return nil, err
	}
	pkce := in.ClientKind == ClientPublic
	if in.PKCERequired != nil {
		pkce = *in.PKCERequired
	}
	if in.ClientKind == ClientPublic && !pkce {
		return nil, fmt.Errorf("%w: public clients require pkceRequired=true", ErrValidation)
	}

	email := strings.TrimSpace(in.PrincipalEmail)
	if email == "" {
		email = fmt.Sprintf("integration+%s@one.local", strings.ReplaceAll(in.APIName, ".", "+"))
	}
	display := strings.TrimSpace(in.PrincipalName)
	if display == "" {
		display = in.Label
	}
	if display == "" {
		display = in.APIName
	}

	u, err := s.users().Create(ctx, db.CreatePrincipalInput{
		Email:         email,
		DisplayName:   display,
		PrincipalType: "service",
	})
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			return nil, fmt.Errorf("%w: principal email already exists", ErrConflict)
		}
		if errors.Is(err, db.ErrValidation) {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		return nil, err
	}

	roles := in.RoleAPINames
	if len(roles) == 0 {
		if in.ClientKind == ClientPublic {
			roles = []string{"StandardUser"}
		} else {
			roles = []string{"StandardUser"}
			for _, f := range in.OAuthFlows {
				if f == identity.FlowAuthorizationCode {
					roles = []string{"MetadataDeveloper"}
					break
				}
			}
		}
	}
	if err := s.users().EnsureSystemRoles(ctx); err != nil {
		return nil, err
	}
	for _, role := range roles {
		if err := s.users().EnsureUserHasRole(ctx, u.ID, role); err != nil {
			return nil, fmt.Errorf("assign role %s: %w", role, err)
		}
	}

	if _, err := s.syncIdentity(ctx, u.ID, in.APIName, in.ClientKind, in.OAuthFlows, in.CallbackURLs, in.LogoutURLs, nil); err != nil {
		return nil, fmt.Errorf("identity sync: %w", err)
	}

	var onePlain, oneEnc string
	if in.ClientKind == ClientConfidential {
		cred, plain, err := s.creds().GenerateClientSecret(ctx, u.ID, in.APIName)
		if err != nil {
			return nil, err
		}
		_ = cred
		onePlain = plain
		enc, err := secretcrypt.Encrypt(plain, s.EncryptionKey)
		if err != nil {
			return nil, err
		}
		oneEnc = enc
	}

	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = in.APIName
	}
	cfg, err := s.store().Insert(ctx, db.CreateIntegrationInput{
		APIName:           in.APIName,
		Label:             label,
		Description:       in.Description,
		PrincipalID:       u.ID,
		ClientKind:        in.ClientKind,
		OAuthFlows:        in.OAuthFlows,
		CallbackURLs:      in.CallbackURLs,
		LogoutURLs:        in.LogoutURLs,
		AllowedScopesHint: in.AllowedScopesHint,
		PKCERequired:      pkce,
		Ownership:         OwnershipCustom,
		PackageName:       "customer.default",
		IsActive:          true,
		OneSecretEnc:      oneEnc,
	})
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			return nil, fmt.Errorf("%w: apiName already exists", ErrConflict)
		}
		if errors.Is(err, db.ErrValidation) {
			return nil, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		return nil, err
	}

	return &CreateResult{
		View:            toView(cfg),
		OneClientSecret: onePlain,
	}, nil
}

// List returns all integrations.
func (s *Service) List(ctx context.Context) ([]View, error) {
	rows, err := s.store().List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for i := range rows {
		out = append(out, toView(&rows[i]))
	}
	return out, nil
}

// Get returns one integration by api_name.
func (s *Service) Get(ctx context.Context, apiName string) (*View, error) {
	c, err := s.store().GetByAPIName(ctx, apiName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	v := toView(c)
	return &v, nil
}

// PatchInput is a partial update.
type PatchInput struct {
	Label             *string
	Description       *string
	OAuthFlows        *[]string
	CallbackURLs      *[]string
	LogoutURLs        *[]string
	AllowedScopesHint *[]string
	AllowedCIDRs      *[]string
	PKCERequired      *bool
	IsActive          *bool
}

// Patch updates an integration. Managed rows only allow callbacks/logout/active/label/description.
func (s *Service) Patch(ctx context.Context, apiName string, in PatchInput) (*View, error) {
	cur, err := s.store().GetByAPIName(ctx, apiName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if cur.Ownership == OwnershipManaged {
		if in.OAuthFlows != nil || in.PKCERequired != nil || in.AllowedScopesHint != nil {
			return nil, fmt.Errorf("%w: cannot change oauth shape on managed integration", ErrForbidden)
		}
	}
	if cur.ClientKind == ClientPublic && in.AllowedScopesHint != nil {
		if err := validatePublicPatchScopes(*in.AllowedScopesHint); err != nil {
			return nil, err
		}
		normalized := normalizePublicScopes(*in.AllowedScopesHint)
		in.AllowedScopesHint = &normalized
	}
	var flows []string
	if in.OAuthFlows != nil {
		flows, err = normalizeFlows(*in.OAuthFlows)
		if err != nil {
			return nil, err
		}
		in.OAuthFlows = &flows
	} else {
		flows = cur.OAuthFlows
	}
	callbacks := cur.CallbackURLs
	if in.CallbackURLs != nil {
		callbacks = *in.CallbackURLs
	}
	logouts := cur.LogoutURLs
	if in.LogoutURLs != nil {
		logouts = *in.LogoutURLs
	}

	patched, err := s.store().Patch(ctx, apiName, db.PatchIntegrationInput{
		Label:             in.Label,
		Description:       in.Description,
		OAuthFlows:        in.OAuthFlows,
		CallbackURLs:      in.CallbackURLs,
		LogoutURLs:        in.LogoutURLs,
		AllowedScopesHint: in.AllowedScopesHint,
		AllowedCIDRs:      in.AllowedCIDRs,
		PKCERequired:      in.PKCERequired,
		IsActive:          in.IsActive,
	})
	if err != nil {
		return nil, err
	}

	if s.Identity != nil && s.Identity.Enabled() {
		clientID, err := s.identityAppClientID(ctx, patched.PrincipalID)
		if err != nil {
			return nil, err
		}
		if clientID != "" {
			if err := s.Identity.UpdateAppClient(ctx, clientID, s.appSpec(patched.APIName, patched.ClientKind, flows, callbacks, logouts)); err != nil {
				return nil, fmt.Errorf("identity update: %w", err)
			}
		}
	}
	v := toView(patched)
	return &v, nil
}

// Delete removes a customer integration, identity app client (when linked), and deactivates the principal.
func (s *Service) Delete(ctx context.Context, apiName string) error {
	cur, err := s.store().GetByAPIName(ctx, apiName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if cur.Ownership == OwnershipManaged {
		return fmt.Errorf("%w: cannot delete managed integration", ErrForbidden)
	}
	if s.Identity != nil && s.Identity.Enabled() {
		if clientID, err := s.identityAppClientID(ctx, cur.PrincipalID); err == nil && clientID != "" {
			_ = s.Identity.DeleteAppClient(ctx, clientID)
		}
	}
	if err := s.store().Delete(ctx, apiName); err != nil {
		return err
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE users SET is_active = false, updated_at = now() WHERE id = $1::uuid`, cur.PrincipalID)
	return nil
}

// RotateResult holds one-time rotated Majesta One secrets.
type RotateResult struct {
	View            View
	OneClientSecret string `json:"oneClientSecret,omitempty"`
}

// Rotate regenerates Majesta One secrets for confidential clients and recreates the identity app client when enabled.
func (s *Service) Rotate(ctx context.Context, apiName string) (*RotateResult, error) {
	cur, err := s.store().GetByAPIName(ctx, apiName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if cur.ClientKind != ClientConfidential {
		return nil, fmt.Errorf("%w: only confidential integrations have rotatable secrets", ErrValidation)
	}

	// Revoke prior Majesta One credentials then issue a new one.
	metas, err := s.creds().ListMetaByUserID(ctx, cur.PrincipalID)
	if err != nil {
		return nil, err
	}
	for _, m := range metas {
		if m.RevokedAt == nil {
			_ = s.creds().Revoke(ctx, cur.PrincipalID, m.ID)
		}
	}
	_, plain, err := s.creds().GenerateClientSecret(ctx, cur.PrincipalID, apiName)
	if err != nil {
		return nil, err
	}
	oneEnc, err := secretcrypt.Encrypt(plain, s.EncryptionKey)
	if err != nil {
		return nil, err
	}

	// Recreate identity app client when sync is on (link stored in identity_links only).
	if s.Identity != nil && s.Identity.Enabled() {
		if existingID, err := s.identityAppClientID(ctx, cur.PrincipalID); err == nil && existingID != "" {
			_ = s.Identity.DeleteAppClient(ctx, existingID)
		}
		id, _, err := s.Identity.CreateAppClient(ctx, s.appSpec(cur.APIName, cur.ClientKind, cur.OAuthFlows, cur.CallbackURLs, cur.LogoutURLs))
		if err != nil {
			return nil, fmt.Errorf("identity recreate: %w", err)
		}
		issuer := strings.TrimSpace(s.IdentityIssuer)
		if _, err := s.links().Upsert(ctx, cur.PrincipalID, identity.ProviderForBackend(s.Identity.Mode()), issuer, id); err != nil {
			return nil, err
		}
	}

	patched, err := s.store().Patch(ctx, apiName, db.PatchIntegrationInput{
		OneSecretEnc: &oneEnc,
	})
	if err != nil {
		return nil, err
	}
	return &RotateResult{View: toView(patched), OneClientSecret: plain}, nil
}

// RevealResult returns decrypted Majesta One secrets for admin retrieve-after.
type RevealResult struct {
	OneClientSecret string `json:"oneClientSecret,omitempty"`
}

// Reveal decrypts stored Majesta One secrets.
func (s *Service) Reveal(ctx context.Context, apiName string) (*RevealResult, error) {
	cur, err := s.store().GetByAPIName(ctx, apiName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := &RevealResult{}
	if cur.OneSecretEnc != nil && *cur.OneSecretEnc != "" {
		plain, err := secretcrypt.Decrypt(*cur.OneSecretEnc, s.EncryptionKey)
		if err != nil {
			return nil, err
		}
		out.OneClientSecret = plain
	}
	if out.OneClientSecret == "" {
		return nil, fmt.Errorf("%w: no secrets stored for reveal", ErrValidation)
	}
	return out, nil
}

// EnsureControlIDE seeds the managed Control IDE public PKCE integration (idempotent).
func (s *Service) EnsureControlIDE(ctx context.Context) error {
	requiredCallbacks := []string{
		"http://127.0.0.1:5173/oauth/callback",
		"http://localhost:5173/oauth/callback",
		"one-control://oauth/callback",
	}
	if existing, err := s.store().GetByAPIName(ctx, APINameControlIDE); err == nil {
		return s.ensureControlIDECallbacks(ctx, existing, requiredCallbacks)
	} else if !errors.Is(err, db.ErrNotFound) {
		return err
	}

	email := "control-ide@one.local"
	display := "Majesta One Control IDE"
	u, err := s.users().GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return err
		}
		u, err = s.users().Create(ctx, db.CreatePrincipalInput{
			Email:         email,
			DisplayName:   display,
			PrincipalType: "service",
		})
		if err != nil {
			return err
		}
	}
	if err := s.users().EnsureSystemRoles(ctx); err != nil {
		return err
	}
	if err := s.users().EnsureUserHasRole(ctx, u.ID, "MetadataDeveloper"); err != nil {
		return err
	}

	flows := []string{identity.FlowAuthorizationCode}
	callbacks := append([]string(nil), requiredCallbacks...)
	logouts := []string{
		"http://127.0.0.1:5173/",
		"http://localhost:5173/",
	}

	if _, err := s.syncIdentity(ctx, u.ID, APINameControlIDE, ClientPublic, flows, callbacks, logouts, nil); err != nil {
		return fmt.Errorf("control ide identity: %w", err)
	}

	pkg := "platform"
	_, err = s.store().Insert(ctx, db.CreateIntegrationInput{
		APIName:           APINameControlIDE,
		Label:             "Majesta One Control IDE",
		Description:       "OOTB public OAuth (authorization_code + PKCE) client for Majesta One Control IDE. Override callback URLs per install via PATCH.",
		PrincipalID:       u.ID,
		ClientKind:        ClientPublic,
		OAuthFlows:        flows,
		CallbackURLs:      callbacks,
		LogoutURLs:        logouts,
		AllowedScopesHint: []string{"openid", "email", "profile", "offline_access"},
		PKCERequired:      true,
		Ownership:         OwnershipManaged,
		PackageName:       pkg,
		IsActive:          true,
	})
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			row, getErr := s.store().GetByAPIName(ctx, APINameControlIDE)
			if getErr != nil {
				return getErr
			}
			return s.ensureControlIDECallbacks(ctx, row, requiredCallbacks)
		}
		return err
	}
	return nil
}

func (s *Service) ensureControlIDECallbacks(ctx context.Context, existing *db.IntegrationConfig, required []string) error {
	have := map[string]struct{}{}
	merged := make([]string, 0, len(existing.CallbackURLs)+len(required))
	for _, c := range existing.CallbackURLs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := have[c]; ok {
			continue
		}
		have[c] = struct{}{}
		merged = append(merged, c)
	}
	cbChanged := false
	for _, c := range required {
		if _, ok := have[c]; ok {
			continue
		}
		have[c] = struct{}{}
		merged = append(merged, c)
		cbChanged = true
	}
	scopes := append([]string(nil), existing.AllowedScopesHint...)
	scopeChanged := !containsScopeHint(scopes, "offline_access")
	if scopeChanged {
		scopes = append(scopes, "offline_access")
	}
	if !cbChanged && !scopeChanged {
		return nil
	}
	in := db.PatchIntegrationInput{}
	if cbChanged {
		in.CallbackURLs = &merged
	}
	if scopeChanged {
		in.AllowedScopesHint = &scopes
	}
	_, err := s.store().Patch(ctx, existing.APIName, in)
	return err
}

func containsScopeHint(hints []string, want string) bool {
	for _, s := range hints {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}
