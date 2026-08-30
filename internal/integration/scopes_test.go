package integration

import "testing"

func TestNormalizePublicScopesDefaultsClient(t *testing.T) {
	got := normalizePublicScopes(nil)
	if len(got) != 1 || got[0] != "client" {
		t.Fatalf("got %v want [client]", got)
	}
}

func TestValidatePublicScopesRejectsMetadata(t *testing.T) {
	err := validatePublicScopes([]string{"client", "metadata"})
	if err == nil {
		t.Fatal("expected error for metadata scope on public client")
	}
}

func TestValidatePublicScopesAllowsOfflineAccess(t *testing.T) {
	if err := validatePublicScopes([]string{"client", "offline_access"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePublicClientRolesStandardUserOnly(t *testing.T) {
	if err := validatePublicClientRoles([]string{"StandardUser"}); err != nil {
		t.Fatal(err)
	}
	if err := validatePublicClientRoles([]string{"MetadataDeveloper"}); err == nil {
		t.Fatal("expected error for MetadataDeveloper on public client")
	}
}

func TestApplyPublicClientDefaults(t *testing.T) {
	in := CreateInput{ClientKind: ClientPublic}
	if err := applyPublicClientDefaults(&in); err != nil {
		t.Fatal(err)
	}
	if len(in.AllowedScopesHint) != 1 || in.AllowedScopesHint[0] != "client" {
		t.Fatalf("scopes %v", in.AllowedScopesHint)
	}
}
