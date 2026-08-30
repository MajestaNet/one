package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "one"

	storeAuto     = "auto"
	storeFile     = "file"
	storeKeychain = "keychain"
)

// secretBackend is the OS credential store (keychain / Secret Service / Credential Manager).
type secretBackend interface {
	Set(service, user, secret string) error
	Get(service, user string) (string, error)
	Delete(service, user string) error
}

type osKeyring struct{}

func (osKeyring) Set(service, user, secret string) error {
	return keyring.Set(service, user, secret)
}

func (osKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (osKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

type secretPayload struct {
	Token  string `json:"token,omitempty"`
	APIKey string `json:"apiKey,omitempty"`
}

var (
	secretsMu sync.Mutex
	secrets   secretBackend = osKeyring{}
)

func setSecretBackendForTest(b secretBackend) func() {
	secretsMu.Lock()
	prev := secrets
	secrets = b
	secretsMu.Unlock()
	return func() {
		secretsMu.Lock()
		secrets = prev
		secretsMu.Unlock()
	}
}

func credentialStoreMode() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ONE_CREDENTIAL_STORE")))
	switch v {
	case storeFile, storeKeychain, storeAuto:
		return v
	default:
		return storeAuto
	}
}

func persistCredential(ref string, c credential) (store string, stored credential, err error) {
	mode := credentialStoreMode()
	payload, err := json.Marshal(secretPayload{Token: c.Token, APIKey: c.APIKey})
	if err != nil {
		return "", credential{}, err
	}
	tryKeychain := mode == storeKeychain || mode == storeAuto
	if tryKeychain {
		secretsMu.Lock()
		setErr := secrets.Set(keyringService, ref, string(payload))
		secretsMu.Unlock()
		if setErr == nil {
			return storeKeychain, credential{Token: c.Token, APIKey: c.APIKey, Backend: storeKeychain}, nil
		}
		if mode == storeKeychain {
			return "", credential{}, fmt.Errorf("OS keychain unavailable: %w", setErr)
		}
	}
	return storeFile, credential{Token: c.Token, APIKey: c.APIKey, Backend: storeFile}, nil
}

func materializeCredential(ref string, stored credential) (credential, error) {
	backend := strings.TrimSpace(stored.Backend)
	if backend == storeKeychain || (backend == "" && stored.Token == "" && stored.APIKey == "") {
		secretsMu.Lock()
		raw, err := secrets.Get(keyringService, ref)
		secretsMu.Unlock()
		if err == nil && raw != "" {
			var p secretPayload
			if json.Unmarshal([]byte(raw), &p) == nil && (p.Token != "" || p.APIKey != "") {
				return credential{Token: p.Token, APIKey: p.APIKey, Backend: storeKeychain}, nil
			}
		}
		if stored.Token != "" || stored.APIKey != "" {
			return stored, nil
		}
		if backend == storeKeychain {
			if err == nil {
				err = errors.New("empty secret")
			}
			return credential{}, fmt.Errorf("OS keychain secret for %s: %w", ref, err)
		}
	}
	return stored, nil
}

func deleteStoredSecret(ref string) {
	secretsMu.Lock()
	_ = secrets.Delete(keyringService, ref)
	secretsMu.Unlock()
}

func stripSecretsForDisk(cred credentialsFile) credentialsFile {
	out := credentialsFile{}
	for k, v := range cred {
		if strings.TrimSpace(v.Backend) == storeKeychain {
			out[k] = credential{Backend: storeKeychain}
			continue
		}
		out[k] = credential{Token: v.Token, APIKey: v.APIKey, Backend: v.Backend}
	}
	return out
}
