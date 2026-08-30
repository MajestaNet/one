package integration_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/integration"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestValidateCreatePublicRequiresCode(t *testing.T) {
	svc := &integration.Service{}
	_, err := svc.Create(t.Context(), integration.CreateInput{
		APIName:    "acme.bot",
		ClientKind: integration.ClientPublic,
		OAuthFlows: []string{identity.FlowClientCredentials},
	})
	if !errors.Is(err, integration.ErrValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestValidateCreateRejectsEmptyAndBadKind(t *testing.T) {
	svc := &integration.Service{}
	_, err := svc.Create(t.Context(), integration.CreateInput{
		ClientKind: integration.ClientConfidential,
		OAuthFlows: []string{identity.FlowClientCredentials},
	})
	if !errors.Is(err, integration.ErrValidation) {
		t.Fatalf("empty apiName: %v", err)
	}
	_, err = svc.Create(t.Context(), integration.CreateInput{
		APIName:    "acme.bot",
		ClientKind: "mystery",
		OAuthFlows: []string{identity.FlowClientCredentials},
	})
	if !errors.Is(err, integration.ErrValidation) {
		t.Fatalf("bad kind: %v", err)
	}
	_, err = svc.Create(t.Context(), integration.CreateInput{
		APIName:    "acme.bot",
		ClientKind: integration.ClientConfidential,
		OAuthFlows: []string{"implicit"},
	})
	if !errors.Is(err, integration.ErrValidation) {
		t.Fatalf("bad flow: %v", err)
	}
}

func TestValidateCreateRejectsOnePrefix(t *testing.T) {
	svc := &integration.Service{}
	_, err := svc.Create(t.Context(), integration.CreateInput{
		APIName:    "one.evil",
		ClientKind: integration.ClientConfidential,
		OAuthFlows: []string{identity.FlowClientCredentials},
	})
	if !errors.Is(err, integration.ErrValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestCreateListGetDeleteWithMemoryBackend(t *testing.T) {
	d := testutil.RequireDatabase(t)
	mem := identity.NewMemoryBackend()
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{
		Identity:      mem,
		EncryptionKey: "test-enc-key-for-integrations!!",
	})
	svc := &integration.Service{
		Pool:          d.Pool,
		Identity:      mem,
		EncryptionKey: "test-enc-key-for-integrations!!",
	}
	apiName := fmt.Sprintf("acme.cover.%d", time.Now().UnixNano())
	created, err := svc.Create(t.Context(), integration.CreateInput{
		APIName:      apiName,
		Label:        "Cover App",
		ClientKind:   integration.ClientConfidential,
		OAuthFlows:   []string{identity.FlowClientCredentials},
		RoleAPINames: []string{"StandardUser"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.OneClientSecret == "" || created.View.APIName != apiName || !created.View.HasOneSecret {
		t.Fatalf("created=%+v", created)
	}
	got, err := svc.Get(t.Context(), apiName)
	if err != nil || got.PrincipalID == "" {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	list, err := svc.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range list {
		if v.APIName == apiName {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("list missing created integration")
	}
	rotated, err := svc.Rotate(t.Context(), apiName)
	if err != nil || rotated.OneClientSecret == "" || rotated.OneClientSecret == created.OneClientSecret {
		t.Fatalf("rotate: %+v err=%v", rotated, err)
	}
	if err := svc.Delete(t.Context(), apiName); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(t.Context(), apiName); !errors.Is(err, integration.ErrNotFound) {
		t.Fatalf("deleted get: %v", err)
	}
	if err := svc.Delete(t.Context(), integration.APINameControlIDE); !errors.Is(err, integration.ErrForbidden) {
		t.Fatalf("managed delete: %v", err)
	}
}
