package identity

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// identity_links.provider values (ADR-015).
const (
	ProviderCognito = "cognito"
	ProviderGoogle  = "google"
	ProviderApple   = "apple"
	ProviderSlack   = "slack"
	ProviderOIDC    = "oidc"
	ProviderMemory  = "memory"
	ProviderDev     = "dev" // local development broker (no external IdP)
)

// ProviderForBackend maps identity.Backend.Mode() to identity_links.provider.
func ProviderForBackend(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "cognito":
		return ProviderCognito
	case "memory":
		return ProviderMemory
	default:
		return ProviderOIDC
	}
}

// FlowAuthorizationCode and FlowClientCredentials are supported OAuth flow names.
const (
	FlowAuthorizationCode = "authorization_code"
	FlowClientCredentials = "client_credentials"
)

// AppClientSpec describes a Cognito User Pool app client for write-through.
type AppClientSpec struct {
	Name           string
	PrincipalType  string // service | agent (label only)
	Confidential   bool
	OAuthFlows     []string // authorization_code, client_credentials
	CallbackURLs   []string
	LogoutURLs     []string
	GenerateSecret bool
}

// DefaultM2MAppClientSpec builds a confidential machine client (legacy principal path).
func DefaultM2MAppClientSpec(name, principalType string) AppClientSpec {
	return AppClientSpec{
		Name:           name,
		PrincipalType:  principalType,
		Confidential:   true,
		OAuthFlows:     []string{FlowClientCredentials},
		GenerateSecret: true,
	}
}

// Backend provisions directory objects in an optional identity adapter (or a memory stand-in).
// AWS Cognito write-through lives in the community module sdk/aws/identity — not the product binary.
type Backend interface {
	Enabled() bool
	Mode() string
	// ProvisionUser creates a directory user. Returns provider subject id.
	ProvisionUser(ctx context.Context, email, displayName string) (sub string, err error)
	// SetUserActive enables or disables a user by username (often email).
	SetUserActive(ctx context.Context, username string, active bool) error
	// CreateAppClient creates an app client. Returns client id + secret (secret empty for public).
	CreateAppClient(ctx context.Context, spec AppClientSpec) (clientID, clientSecret string, err error)
	// UpdateAppClient updates an existing app client.
	UpdateAppClient(ctx context.Context, clientID string, spec AppClientSpec) error
	// DeleteAppClient removes an app client by id.
	DeleteAppClient(ctx context.Context, clientID string) error
}

// NewBackendFromConfig returns MemoryBackend when syncMode is "memory", else NopBackend.
// Cognito / other cloud adapters are not wired in the default product binary; see sdk/aws.
func NewBackendFromConfig(syncMode string) Backend {
	switch strings.ToLower(strings.TrimSpace(syncMode)) {
	case "memory":
		return NewMemoryBackend()
	default:
		return NopBackend{}
	}
}

// MemoryBackend records provisions for Compose/tests when no external identity adapter is configured.
type MemoryBackend struct {
	mu      sync.Mutex
	Users   map[string]memUser // email -> user
	Clients map[string]memClient
	seq     int
}

type memUser struct {
	Sub         string
	Email       string
	DisplayName string
	Active      bool
}

type memClient struct {
	ID            string
	Secret        string
	Name          string
	PrincipalType string
	Spec          AppClientSpec
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		Users:   map[string]memUser{},
		Clients: map[string]memClient{},
	}
}

func (m *MemoryBackend) Enabled() bool { return true }
func (m *MemoryBackend) Mode() string  { return "memory" }

func (m *MemoryBackend) ProvisionUser(_ context.Context, email, displayName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", fmt.Errorf("email required")
	}
	if u, ok := m.Users[email]; ok {
		return u.Sub, nil
	}
	m.seq++
	sub := fmt.Sprintf("mem-user-%d", m.seq)
	m.Users[email] = memUser{Sub: sub, Email: email, DisplayName: displayName, Active: true}
	return sub, nil
}

func (m *MemoryBackend) SetUserActive(_ context.Context, username string, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	username = strings.ToLower(strings.TrimSpace(username))
	u, ok := m.Users[username]
	if !ok {
		return fmt.Errorf("user not found")
	}
	u.Active = active
	m.Users[username] = u
	return nil
}

func (m *MemoryBackend) CreateAppClient(_ context.Context, spec AppClientSpec) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// UUIDs avoid identity_links unique collisions when tests share one Postgres
	// and each package constructs a fresh MemoryBackend (seq would restart at 1).
	id := "mem-client-" + uuid.NewString()
	secret := ""
	if spec.GenerateSecret || spec.Confidential {
		secret = "mem-secret-" + uuid.NewString()
	}
	m.Clients[id] = memClient{
		ID: id, Secret: secret, Name: spec.Name, PrincipalType: spec.PrincipalType, Spec: spec,
	}
	return id, secret, nil
}

func (m *MemoryBackend) UpdateAppClient(_ context.Context, clientID string, spec AppClientSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.Clients[clientID]
	if !ok {
		return fmt.Errorf("app client not found")
	}
	c.Name = spec.Name
	c.PrincipalType = spec.PrincipalType
	c.Spec = spec
	m.Clients[clientID] = c
	return nil
}

func (m *MemoryBackend) DeleteAppClient(_ context.Context, clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Clients, clientID)
	return nil
}

// NopBackend disables external identity write-through.
type NopBackend struct{}

func (NopBackend) Enabled() bool { return false }
func (NopBackend) Mode() string  { return "off" }
func (NopBackend) ProvisionUser(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("identity backend disabled")
}
func (NopBackend) SetUserActive(context.Context, string, bool) error {
	return fmt.Errorf("identity backend disabled")
}
func (NopBackend) CreateAppClient(context.Context, AppClientSpec) (string, string, error) {
	return "", "", fmt.Errorf("identity backend disabled")
}
func (NopBackend) UpdateAppClient(context.Context, string, AppClientSpec) error {
	return fmt.Errorf("identity backend disabled")
}
func (NopBackend) DeleteAppClient(context.Context, string) error {
	return fmt.Errorf("identity backend disabled")
}
