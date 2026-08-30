package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListenAddrDualStack(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host string
		port int
		want string
	}{
		{host: "", port: 8080, want: ":8080"},
		{host: "0.0.0.0", port: 8080, want: ":8080"},
		{host: "::", port: 8080, want: ":8080"},
		{host: "[::]", port: 9090, want: ":9090"},
		{host: "127.0.0.1", port: 8080, want: "127.0.0.1:8080"},
		{host: "::1", port: 8080, want: "[::1]:8080"},
		{host: "localhost", port: 8080, want: "localhost:8080"},
	}
	for _, tc := range cases {
		got := ListenAddr(tc.host, tc.port)
		if got != tc.want {
			t.Fatalf("ListenAddr(%q, %d)=%q want %q", tc.host, tc.port, got, tc.want)
		}
	}
	cfg := &Config{Host: "0.0.0.0", Port: 8080}
	if got := cfg.ListenAddr(); got != ":8080" {
		t.Fatalf("Config.ListenAddr=%q", got)
	}
}

func TestListenAddrAcceptsLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", ListenAddr("0.0.0.0", 0))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	addr := ln.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}

	dial := func(host string) {
		t.Helper()
		c, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
		if err != nil {
			t.Fatalf("dial %s: %v", host, err)
		}
		_ = c.Close()
	}
	dial("127.0.0.1")
	dial("localhost")
}

func TestParseDotEnv(t *testing.T) {
	t.Parallel()
	got := parseDotEnv(`
# comment
DATABASE_URL=postgres://one:one@localhost:5432/one
export PORT=8080
AUTH_JWT_SIGNING_KEY="quoted secret"
EMPTY=
`)
	if got["DATABASE_URL"] != "postgres://one:one@localhost:5432/one" {
		t.Fatalf("DATABASE_URL=%q", got["DATABASE_URL"])
	}
	if got["PORT"] != "8080" {
		t.Fatalf("PORT=%q", got["PORT"])
	}
	if got["AUTH_JWT_SIGNING_KEY"] != "quoted secret" {
		t.Fatalf("AUTH_JWT_SIGNING_KEY=%q", got["AUTH_JWT_SIGNING_KEY"])
	}
	if got["EMPTY"] != "" {
		t.Fatalf("EMPTY=%q", got["EMPTY"])
	}
}

func TestApplyDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("ONE_DOTENV_KEEP=from-file\nONE_DOTENV_FRESH=yes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONE_DOTENV_KEEP", "from-process")
	_ = os.Unsetenv("ONE_DOTENV_FRESH")

	ApplyDotEnv()
	if got := os.Getenv("ONE_DOTENV_KEEP"); got != "from-process" {
		t.Fatalf("KEEP=%q, want process value", got)
	}
	if got := os.Getenv("ONE_DOTENV_FRESH"); got != "yes" {
		t.Fatalf("FRESH=%q, want value from .env", got)
	}
}
