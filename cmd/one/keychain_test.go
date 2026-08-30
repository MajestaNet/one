package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type memKeyring struct {
	mu   sync.Mutex
	data map[string]string
	fail error
}

func (m *memKeyring) k(service, user string) string { return service + "\x00" + user }

func (m *memKeyring) Set(service, user, secret string) error {
	if m.fail != nil {
		return m.fail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string]string{}
	}
	m.data[m.k(service, user)] = secret
	return nil
}

func (m *memKeyring) Get(service, user string) (string, error) {
	if m.fail != nil {
		return "", m.fail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[m.k(service, user)]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (m *memKeyring) Delete(service, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, m.k(service, user))
	return nil
}

func TestPersistCredentialKeychainOmitsPlaintext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ONE_CONFIG_DIR", dir)
	t.Setenv("ONE_CREDENTIAL_STORE", "keychain")
	kr := &memKeyring{}
	restore := setSecretBackendForTest(kr)
	t.Cleanup(restore)
	resetConfigCacheForTest()

	store, stored, err := persistCredential("test", credential{Token: "secret-jwt"})
	if err != nil {
		t.Fatal(err)
	}
	if store != storeKeychain || stored.Backend != storeKeychain || stored.Token != "secret-jwt" {
		t.Fatalf("stored=%+v store=%s", stored, store)
	}
	cfg := &cliConfig{
		DefaultOrg: "test",
		Orgs:       map[string]orgConfig{"test": {BaseURL: "https://t.example", CredentialRef: "test"}},
	}
	if err := saveConfig(cfg, credentialsFile{"test": stored}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-jwt") {
		t.Fatalf("credentials.json must not contain the JWT: %s", raw)
	}
	var disk credentialsFile
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if disk["test"].Backend != storeKeychain || disk["test"].Token != "" {
		t.Fatalf("disk=%+v", disk["test"])
	}

	resetConfigCacheForTest()
	_, cred, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cred["test"].Token != "secret-jwt" {
		t.Fatalf("materialize=%+v", cred["test"])
	}
}

func TestPersistCredentialAutoFallsBackToFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ONE_CONFIG_DIR", dir)
	t.Setenv("ONE_CREDENTIAL_STORE", "auto")
	restore := setSecretBackendForTest(&memKeyring{fail: errors.New("no dbus")})
	t.Cleanup(restore)
	resetConfigCacheForTest()

	store, stored, err := persistCredential("test", credential{Token: "fallback-jwt"})
	if err != nil {
		t.Fatal(err)
	}
	if store != storeFile || stored.Backend != storeFile {
		t.Fatalf("expected file fallback, got store=%s %+v", store, stored)
	}
	if err := saveConfig(&cliConfig{Orgs: map[string]orgConfig{}}, credentialsFile{"test": stored}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "fallback-jwt") {
		t.Fatalf("file store should persist token: %s", raw)
	}
}

func TestPersistCredentialKeychainRequired(t *testing.T) {
	t.Setenv("ONE_CREDENTIAL_STORE", "keychain")
	restore := setSecretBackendForTest(&memKeyring{fail: errors.New("no keychain")})
	t.Cleanup(restore)
	if _, _, err := persistCredential("x", credential{Token: "t"}); err == nil {
		t.Fatal("expected error when keychain is required")
	}
}

func TestLogoutDeletesKeychainSecret(t *testing.T) {
	t.Setenv("ONE_CREDENTIAL_STORE", "keychain")
	kr := &memKeyring{}
	restore := setSecretBackendForTest(kr)
	t.Cleanup(restore)
	if _, stored, err := persistCredential("acme", credential{APIKey: "k"}); err != nil {
		t.Fatal(err)
	} else if stored.Backend != storeKeychain {
		t.Fatalf("%+v", stored)
	}
	deleteStoredSecret("acme")
	if _, err := kr.Get(keyringService, "acme"); err == nil {
		t.Fatal("expected keychain delete")
	}
}
