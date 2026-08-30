// Command validate-do-app-spec checks Majesta One App Platform YAML has required shape.
// Usage: go run ./scripts/validate-do-app-spec.go [-strict-digest] [path...]
// Default path: deploy/digitalocean/app.yaml
//
// The checked-in example may omit image digest pins. Use -strict-digest on
// operator copies after scripts/apply-do-app-digests.sh.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type appSpec struct {
	Name     string          `yaml:"name"`
	Region   string          `yaml:"region"`
	Services []componentSpec `yaml:"services"`
	Workers  []componentSpec `yaml:"workers"`
}

type componentSpec struct {
	Name             string     `yaml:"name"`
	Image            *imageSpec `yaml:"image"`
	HTTPPort         int        `yaml:"http_port"`
	InstanceCount    int        `yaml:"instance_count"`
	InstanceSizeSlug string     `yaml:"instance_size_slug"`
	Envs             []envSpec  `yaml:"envs"`
}

type imageSpec struct {
	RegistryType string `yaml:"registry_type"`
	Registry     string `yaml:"registry"`
	Repository   string `yaml:"repository"`
	Tag          string `yaml:"tag"`
	Digest       string `yaml:"digest"`
}

type envSpec struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
	Type  string `yaml:"type"`
}

type validateOptions struct {
	strictDigest bool
}

var sha256Digest = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)

func main() {
	strictDigest := flag.Bool("strict-digest", false, "require sha256 image digest pins on api and worker")
	flag.Parse()
	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"deploy/digitalocean/app.yaml"}
	}
	opts := validateOptions{strictDigest: *strictDigest}
	failed := false
	for _, p := range paths {
		if err := validate(p, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			failed = true
			continue
		}
		fmt.Printf("%s: ok\n", p)
	}
	if failed {
		os.Exit(1)
	}
}

func validate(path string, opts validateOptions) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return validateBytes(raw, opts)
}

func validateBytes(raw []byte, opts validateOptions) error {
	var spec appSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	if spec.Name == "" {
		return fmt.Errorf("missing name")
	}
	if spec.Region == "" {
		return fmt.Errorf("missing region")
	}
	if len(spec.Services) == 0 {
		return fmt.Errorf("services required (api)")
	}
	if len(spec.Workers) == 0 {
		return fmt.Errorf("workers required (worker)")
	}
	apiOK, workerOK := false, false
	for _, s := range spec.Services {
		if err := checkComponent("service", s, true); err != nil {
			return err
		}
		if s.Name == "api" {
			apiOK = true
			if err := checkAPIEnvs(s); err != nil {
				return err
			}
			if opts.strictDigest {
				if err := checkPinnedDigest("service", s); err != nil {
					return err
				}
			}
		}
	}
	for _, w := range spec.Workers {
		if err := checkComponent("worker", w, false); err != nil {
			return err
		}
		if w.Name == "worker" {
			workerOK = true
			if opts.strictDigest {
				if err := checkPinnedDigest("worker", w); err != nil {
					return err
				}
			}
		}
	}
	if !apiOK {
		return fmt.Errorf("service named api required")
	}
	if !workerOK {
		return fmt.Errorf("worker named worker required")
	}
	return nil
}

func checkComponent(kind string, c componentSpec, needHTTP bool) error {
	if c.Name == "" {
		return fmt.Errorf("%s missing name", kind)
	}
	if c.Image == nil {
		return fmt.Errorf("%s %s missing image", kind, c.Name)
	}
	if c.Image.RegistryType == "" || c.Image.Repository == "" {
		return fmt.Errorf("%s %s image.registry_type and image.repository required", kind, c.Name)
	}
	if c.Image.Tag == "latest" || c.Image.Tag == "" {
		return fmt.Errorf("%s %s must pin image.tag (not empty or latest)", kind, c.Name)
	}
	if c.InstanceCount < 1 {
		return fmt.Errorf("%s %s instance_count must be >= 1", kind, c.Name)
	}
	if c.InstanceSizeSlug == "" {
		return fmt.Errorf("%s %s instance_size_slug required", kind, c.Name)
	}
	if needHTTP && c.HTTPPort == 0 {
		return fmt.Errorf("%s %s http_port required", kind, c.Name)
	}
	required := map[string]bool{"CUSTOMER_ID": false, "INSTALL_ID": false, "INSTALL_ROLE": false, "PRODUCT_VERSION": false, "API_REVISION_CURRENT": false, "API_REVISION_MIN": false, "DATABASE_URL": false}
	for _, e := range c.Envs {
		if _, ok := required[e.Key]; ok {
			required[e.Key] = true
		}
		if e.Key == "DATABASE_URL" && e.Type != "SECRET" {
			return fmt.Errorf("%s %s DATABASE_URL must be type SECRET", kind, c.Name)
		}
	}
	for k, ok := range required {
		if !ok {
			return fmt.Errorf("%s %s missing env %s", kind, c.Name, k)
		}
	}
	return nil
}

func checkAPIEnvs(c componentSpec) error {
	found := map[string]envSpec{}
	for _, e := range c.Envs {
		found[e.Key] = e
	}
	if _, ok := found["PLATFORM_PUBLIC_URL"]; !ok {
		return fmt.Errorf("service api missing env PLATFORM_PUBLIC_URL")
	}
	for _, key := range []string{"API_KEYS", "AUTH_JWT_SIGNING_KEY", "INSTALL_CLAIM_TOKEN"} {
		e, ok := found[key]
		if !ok {
			return fmt.Errorf("service api missing env %s", key)
		}
		if e.Type != "SECRET" {
			return fmt.Errorf("service api %s must be type SECRET", key)
		}
	}
	return nil
}

func checkPinnedDigest(kind string, c componentSpec) error {
	if c.Image == nil {
		return fmt.Errorf("%s %s missing image", kind, c.Name)
	}
	digest := strings.TrimSpace(c.Image.Digest)
	if digest == "" {
		return fmt.Errorf("%s %s image.digest required (-strict-digest)", kind, c.Name)
	}
	if !sha256Digest.MatchString(digest) {
		return fmt.Errorf("%s %s image.digest must be sha256:<64 hex> (-strict-digest)", kind, c.Name)
	}
	return nil
}
