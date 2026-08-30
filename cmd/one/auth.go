package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func cmdAuth(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: one auth <login|logout>\n")
		os.Exit(2)
	}
	switch args[0] {
	case "login":
		cmdAuthLogin(args[1:])
	case "logout":
		cmdAuthLogout(args[1:])
	default:
		fatal(fmt.Errorf("unknown auth subcommand %q", args[0]))
	}
}

func cmdAuthLogin(args []string) {
	fs := flag.NewFlagSet("auth login", flag.ExitOnError)
	alias := fs.String("alias", "default", "org alias")
	baseURL := fs.String("base-url", "", "install API base URL")
	token := fs.String("token", "", "Majesta One JWT")
	apiKey := fs.String("api-key", "", "API key")
	asDefault := fs.Bool("default", true, "set as default org")
	apiRevision := fs.Int("api-revision", 0, "override API revision pin (must be within install window)")
	forceCompat := fs.Bool("force-compat", false, "break-glass: connect despite revision compat block")
	_ = fs.Parse(args)
	if strings.TrimSpace(*baseURL) == "" {
		fatal(fmt.Errorf("--base-url is required"))
	}
	if *token == "" && *apiKey == "" {
		fatal(fmt.Errorf("--token or --api-key is required"))
	}
	pin, probe, err := negotiateCliPin(*baseURL, *forceCompat, *apiRevision)
	if err != nil {
		if *forceCompat {
			fmt.Fprintf(os.Stderr, "warning: compat negotiation failed: %v\n", err)
			if pin <= 0 && probe != nil {
				pin = probe.ApiRevision.Min
			}
			if pin <= 0 {
				pin = 1
			}
		} else {
			compatExit(3, "%v", err)
		}
	}
	cfg, cred, err := loadConfig()
	if err != nil {
		fatal(err)
	}
	if cfg.Orgs == nil {
		cfg.Orgs = map[string]orgConfig{}
	}
	if cred == nil {
		cred = credentialsFile{}
	}
	ref := *alias
	cfg.Orgs[*alias] = orgConfig{
		BaseURL:        strings.TrimRight(*baseURL, "/"),
		CredentialRef:  ref,
		ApiRevisionPin: pin,
	}
	c := credential{}
	if *token != "" {
		c.Token = *token
	} else {
		c.APIKey = *apiKey
	}
	store, stored, err := persistCredential(ref, c)
	if err != nil {
		fatal(err)
	}
	cred[ref] = stored
	if *asDefault {
		cfg.DefaultOrg = *alias
	}
	if err := saveConfig(cfg, cred); err != nil {
		fatal(err)
	}
	fmt.Printf("logged in alias=%s baseUrl=%s apiRevisionPin=%d default=%v store=%s\n", *alias, cfg.Orgs[*alias].BaseURL, pin, *asDefault, store)
}

func cmdAuthLogout(args []string) {
	fs := flag.NewFlagSet("auth logout", flag.ExitOnError)
	alias := fs.String("alias", "", "org alias (default: current defaultOrg)")
	_ = fs.Parse(args)
	cfg, cred, err := loadConfig()
	if err != nil {
		fatal(err)
	}
	a := *alias
	if a == "" {
		a = cfg.DefaultOrg
	}
	if a == "" {
		fatal(fmt.Errorf("no alias to logout"))
	}
	if o, ok := cfg.Orgs[a]; ok {
		deleteStoredSecret(o.CredentialRef)
		deleteStoredSecret(a)
		delete(cred, o.CredentialRef)
		delete(cred, a)
		delete(cfg.Orgs, a)
	}
	if cfg.DefaultOrg == a {
		cfg.DefaultOrg = ""
		for k := range cfg.Orgs {
			cfg.DefaultOrg = k
			break
		}
	}
	if err := saveConfig(cfg, cred); err != nil {
		fatal(err)
	}
	fmt.Printf("logged out alias=%s\n", a)
}
