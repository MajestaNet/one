package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type orgConfig struct {
	BaseURL        string `json:"baseUrl"`
	CredentialRef  string `json:"credentialRef"`
	ApiRevisionPin int    `json:"apiRevisionPin,omitempty"`
}

type cliConfig struct {
	DefaultOrg string               `json:"defaultOrg"`
	Orgs       map[string]orgConfig `json:"orgs"`
}

type credential struct {
	Token   string `json:"token,omitempty"`
	APIKey  string `json:"apiKey,omitempty"`
	Backend string `json:"backend,omitempty"` // keychain | file
}

type credentialsFile map[string]credential

var (
	configMu     sync.Mutex
	cachedCfg    *cliConfig
	cachedCred   credentialsFile
	configLoaded bool
	configErr    error
)

func resetConfigCacheForTest() {
	configMu.Lock()
	defer configMu.Unlock()
	cachedCfg = nil
	cachedCred = nil
	configLoaded = false
	configErr = nil
}

func configDir() (string, error) {
	if v := os.Getenv("ONE_CONFIG_DIR"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "one"), nil
}

func loadConfig() (*cliConfig, credentialsFile, error) {
	configMu.Lock()
	defer configMu.Unlock()
	if configLoaded {
		return cachedCfg, cachedCred, configErr
	}
	configLoaded = true
	dir, err := configDir()
	if err != nil {
		configErr = err
		return nil, nil, err
	}
	cachedCfg = &cliConfig{Orgs: map[string]orgConfig{}}
	cachedCred = credentialsFile{}
	cfgPath := filepath.Join(dir, "config.json")
	if b, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(b, cachedCfg); err != nil {
			configErr = fmt.Errorf("parse config.json: %w", err)
			return nil, nil, configErr
		}
		if cachedCfg.Orgs == nil {
			cachedCfg.Orgs = map[string]orgConfig{}
		}
	} else if !os.IsNotExist(err) {
		configErr = err
		return nil, nil, err
	}
	credPath := filepath.Join(dir, "credentials.json")
	if b, err := os.ReadFile(credPath); err == nil {
		if err := json.Unmarshal(b, &cachedCred); err != nil {
			configErr = fmt.Errorf("parse credentials.json: %w", err)
			return nil, nil, configErr
		}
		for ref, stored := range cachedCred {
			got, merr := materializeCredential(ref, stored)
			if merr != nil {
				configErr = merr
				return nil, nil, configErr
			}
			cachedCred[ref] = got
		}
	} else if !os.IsNotExist(err) {
		configErr = err
		return nil, nil, err
	}
	return cachedCfg, cachedCred, nil
}

func saveConfig(cfg *cliConfig, cred credentialsFile) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if cfg.Orgs == nil {
		cfg.Orgs = map[string]orgConfig{}
	}
	cb, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), append(cb, '\n'), 0o600); err != nil {
		return err
	}
	if cred == nil {
		cred = credentialsFile{}
	}
	disk := stripSecretsForDisk(cred)
	rb, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), append(rb, '\n'), 0o600); err != nil {
		return err
	}
	configMu.Lock()
	cachedCfg = cfg
	cachedCred = cred
	configLoaded = true
	configErr = nil
	configMu.Unlock()
	return nil
}

// resolvedOrg is auth + base URL for an org command.
type resolvedOrg struct {
	Alias          string
	BaseURL        string
	Token          string
	APIKey         string
	ApiRevisionPin int
}

func (r *resolvedOrg) bearer() string {
	if r.Token != "" {
		return r.Token
	}
	return r.APIKey
}

// resolveOrgAuth merges flags, env, and config. Flags win over env over config.
func resolveOrgAuth(alias, baseURL, token, apiKey string) (*resolvedOrg, error) {
	cfg, cred, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if alias == "" {
		alias = os.Getenv("ONE_ORG")
	}
	if alias == "" {
		alias = cfg.DefaultOrg
	}
	out := &resolvedOrg{Alias: alias}
	if alias != "" {
		if o, ok := cfg.Orgs[alias]; ok {
			out.BaseURL = o.BaseURL
			out.ApiRevisionPin = o.ApiRevisionPin
			if c, ok := cred[o.CredentialRef]; ok {
				out.Token = c.Token
				out.APIKey = c.APIKey
			} else if c, ok := cred[alias]; ok {
				out.Token = c.Token
				out.APIKey = c.APIKey
			}
		}
	}
	if v := os.Getenv("ONE_BASE_URL"); v != "" {
		out.BaseURL = v
	}
	if v := os.Getenv("ONE_TOKEN"); v != "" {
		out.Token = v
		out.APIKey = ""
	}
	if v := os.Getenv("ONE_API_KEY"); v != "" && out.Token == "" {
		out.APIKey = v
	}
	if baseURL != "" {
		out.BaseURL = baseURL
	}
	if token != "" {
		out.Token = token
		out.APIKey = ""
	}
	if apiKey != "" && out.Token == "" {
		out.APIKey = apiKey
	}
	if out.BaseURL == "" {
		return nil, fmt.Errorf("base URL required (--base-url, ONE_BASE_URL, or auth login --alias)")
	}
	if out.bearer() == "" {
		return nil, fmt.Errorf("credential required (--token/--api-key, ONE_TOKEN/ONE_API_KEY, or auth login)")
	}
	return out, nil
}
