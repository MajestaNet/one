package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackSelectiveCLI(t *testing.T) {
	t.Setenv("ONE_CONFIG_DIR", t.TempDir())
	resetConfigCacheForTest()
	t.Setenv("ONE_BASE_URL", "https://test.example")
	t.Setenv("ONE_TOKEN", "tok")
	auth, err := resolveOrgAuth("", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if auth.BaseURL != "https://test.example" || auth.bearer() != "tok" {
		t.Fatalf("%+v", auth)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ONE_CONFIG_DIR", dir)
	resetConfigCacheForTest()
	cfg := &cliConfig{
		DefaultOrg: "test",
		Orgs: map[string]orgConfig{
			"test": {BaseURL: "https://t.example", CredentialRef: "test"},
		},
	}
	cred := credentialsFile{"test": {Token: "abc"}}
	if err := saveConfig(cfg, cred); err != nil {
		t.Fatal(err)
	}
	resetConfigCacheForTest()
	loaded, c2, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultOrg != "test" || c2["test"].Token != "abc" {
		t.Fatalf("%+v %+v", loaded, c2)
	}
	st, err := os.Stat(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("credentials should not be group/world readable: %v", st.Mode())
	}
}
