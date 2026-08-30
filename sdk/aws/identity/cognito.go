package identity

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

// CognitoConfig configures AWS Cognito write-through.
type CognitoConfig struct {
	UserPoolID string
	Region     string
	// Issuer is the OIDC issuer (used for identity_links.issuer).
	Issuer string
}

// Configured reports whether Cognito admin APIs can run.
func (c CognitoConfig) Configured() bool {
	return strings.TrimSpace(c.UserPoolID) != ""
}

// CognitoAPI is the subset used by CognitoBackend (mockable).
type CognitoAPI interface {
	AdminCreateUser(ctx context.Context, params *cognitoidentityprovider.AdminCreateUserInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminCreateUserOutput, error)
	AdminDisableUser(ctx context.Context, params *cognitoidentityprovider.AdminDisableUserInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminDisableUserOutput, error)
	AdminEnableUser(ctx context.Context, params *cognitoidentityprovider.AdminEnableUserInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminEnableUserOutput, error)
	CreateUserPoolClient(ctx context.Context, params *cognitoidentityprovider.CreateUserPoolClientInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.CreateUserPoolClientOutput, error)
	UpdateUserPoolClient(ctx context.Context, params *cognitoidentityprovider.UpdateUserPoolClientInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.UpdateUserPoolClientOutput, error)
	DeleteUserPoolClient(ctx context.Context, params *cognitoidentityprovider.DeleteUserPoolClientInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.DeleteUserPoolClientOutput, error)
}

// CognitoBackend provisions Cognito users and app clients.
type CognitoBackend struct {
	Config CognitoConfig
	Client CognitoAPI
}

func (c *CognitoBackend) Enabled() bool { return c != nil && c.Config.Configured() }
func (c *CognitoBackend) Mode() string  { return "cognito" }

func (c *CognitoBackend) client(ctx context.Context) (CognitoAPI, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(c.Config.Region))
	if err != nil {
		return nil, err
	}
	return cognitoidentityprovider.NewFromConfig(cfg), nil
}

func (c *CognitoBackend) ProvisionUser(ctx context.Context, email, displayName string) (string, error) {
	cli, err := c.client(ctx)
	if err != nil {
		return "", err
	}
	email = strings.TrimSpace(email)
	attrs := []types.AttributeType{
		{Name: aws.String("email"), Value: aws.String(email)},
		{Name: aws.String("email_verified"), Value: aws.String("true")},
	}
	if displayName != "" {
		attrs = append(attrs, types.AttributeType{Name: aws.String("name"), Value: aws.String(displayName)})
	}
	out, err := cli.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId:     aws.String(c.Config.UserPoolID),
		Username:       aws.String(email),
		UserAttributes: attrs,
		MessageAction:  types.MessageActionTypeSuppress, // Admin UI / passwordless invite owns messaging
	})
	if err != nil {
		return "", err
	}
	if out.User == nil || out.User.Username == nil {
		return "", fmt.Errorf("cognito AdminCreateUser returned empty user")
	}
	// Prefer sub attribute when present.
	for _, a := range out.User.Attributes {
		if a.Name != nil && *a.Name == "sub" && a.Value != nil {
			return *a.Value, nil
		}
	}
	return *out.User.Username, nil
}

func (c *CognitoBackend) SetUserActive(ctx context.Context, username string, active bool) error {
	cli, err := c.client(ctx)
	if err != nil {
		return err
	}
	if active {
		_, err = cli.AdminEnableUser(ctx, &cognitoidentityprovider.AdminEnableUserInput{
			UserPoolId: aws.String(c.Config.UserPoolID),
			Username:   aws.String(username),
		})
		return err
	}
	_, err = cli.AdminDisableUser(ctx, &cognitoidentityprovider.AdminDisableUserInput{
		UserPoolId: aws.String(c.Config.UserPoolID),
		Username:   aws.String(username),
	})
	return err
}

func clientName(spec AppClientSpec) string {
	label := strings.TrimSpace(spec.Name)
	if label == "" {
		label = spec.PrincipalType
	}
	if label == "" {
		label = "client"
	}
	pt := strings.TrimSpace(spec.PrincipalType)
	if pt == "" {
		pt = "service"
	}
	return fmt.Sprintf("one-%s-%s", pt, label)
}

func wantsHostedUI(spec AppClientSpec) bool {
	for _, f := range spec.OAuthFlows {
		if f == FlowAuthorizationCode {
			return true
		}
	}
	return false
}

func (c *CognitoBackend) CreateAppClient(ctx context.Context, spec AppClientSpec) (string, string, error) {
	cli, err := c.client(ctx)
	if err != nil {
		return "", "", err
	}
	genSecret := spec.GenerateSecret || spec.Confidential
	in := &cognitoidentityprovider.CreateUserPoolClientInput{
		UserPoolId:     aws.String(c.Config.UserPoolID),
		ClientName:     aws.String(clientName(spec)),
		GenerateSecret: genSecret,
		ExplicitAuthFlows: []types.ExplicitAuthFlowsType{
			types.ExplicitAuthFlowsTypeAllowRefreshTokenAuth,
		},
		PreventUserExistenceErrors: types.PreventUserExistenceErrorTypesEnabled,
		EnableTokenRevocation:      aws.Bool(true),
	}
	if wantsHostedUI(spec) {
		in.AllowedOAuthFlowsUserPoolClient = true
		in.AllowedOAuthFlows = []types.OAuthFlowType{types.OAuthFlowTypeCode}
		in.AllowedOAuthScopes = []string{"openid", "email", "profile"}
		if len(spec.CallbackURLs) > 0 {
			in.CallbackURLs = append([]string{}, spec.CallbackURLs...)
		}
		if len(spec.LogoutURLs) > 0 {
			in.LogoutURLs = append([]string{}, spec.LogoutURLs...)
		}
		in.SupportedIdentityProviders = []string{"COGNITO"}
		if !genSecret {
			in.ExplicitAuthFlows = append(in.ExplicitAuthFlows, types.ExplicitAuthFlowsTypeAllowUserSrpAuth)
		}
	}
	out, err := cli.CreateUserPoolClient(ctx, in)
	if err != nil {
		return "", "", err
	}
	if out.UserPoolClient == nil || out.UserPoolClient.ClientId == nil {
		return "", "", fmt.Errorf("cognito CreateUserPoolClient returned empty client")
	}
	secret := ""
	if out.UserPoolClient.ClientSecret != nil {
		secret = *out.UserPoolClient.ClientSecret
	}
	return *out.UserPoolClient.ClientId, secret, nil
}

func (c *CognitoBackend) UpdateAppClient(ctx context.Context, clientID string, spec AppClientSpec) error {
	cli, err := c.client(ctx)
	if err != nil {
		return err
	}
	in := &cognitoidentityprovider.UpdateUserPoolClientInput{
		UserPoolId: aws.String(c.Config.UserPoolID),
		ClientId:   aws.String(clientID),
		ClientName: aws.String(clientName(spec)),
		ExplicitAuthFlows: []types.ExplicitAuthFlowsType{
			types.ExplicitAuthFlowsTypeAllowRefreshTokenAuth,
		},
		PreventUserExistenceErrors: types.PreventUserExistenceErrorTypesEnabled,
		EnableTokenRevocation:      aws.Bool(true),
	}
	if wantsHostedUI(spec) {
		in.AllowedOAuthFlowsUserPoolClient = true
		in.AllowedOAuthFlows = []types.OAuthFlowType{types.OAuthFlowTypeCode}
		in.AllowedOAuthScopes = []string{"openid", "email", "profile"}
		if len(spec.CallbackURLs) > 0 {
			in.CallbackURLs = append([]string{}, spec.CallbackURLs...)
		}
		if len(spec.LogoutURLs) > 0 {
			in.LogoutURLs = append([]string{}, spec.LogoutURLs...)
		}
		in.SupportedIdentityProviders = []string{"COGNITO"}
		if !spec.Confidential && !spec.GenerateSecret {
			in.ExplicitAuthFlows = append(in.ExplicitAuthFlows, types.ExplicitAuthFlowsTypeAllowUserSrpAuth)
		}
	} else {
		in.AllowedOAuthFlowsUserPoolClient = false
	}
	_, err = cli.UpdateUserPoolClient(ctx, in)
	return err
}

func (c *CognitoBackend) DeleteAppClient(ctx context.Context, clientID string) error {
	cli, err := c.client(ctx)
	if err != nil {
		return err
	}
	_, err = cli.DeleteUserPoolClient(ctx, &cognitoidentityprovider.DeleteUserPoolClientInput{
		UserPoolId: aws.String(c.Config.UserPoolID),
		ClientId:   aws.String(clientID),
	})
	return err
}

// NewCognitoBackend returns a CognitoBackend when cfg is configured, else nil.
// Memory/Nop backends stay in the product tree — this module only ships Cognito.
func NewCognitoBackend(cfg CognitoConfig) *CognitoBackend {
	if !cfg.Configured() {
		return nil
	}
	return &CognitoBackend{Config: cfg}
}
