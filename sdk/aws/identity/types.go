package identity

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
