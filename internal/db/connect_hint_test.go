package db_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
)

func TestLocalUnreachableHintLoopbackRefused(t *testing.T) {
	err := errors.New("ping: failed to connect to `user=one database=one`:\n\t127.0.0.1:5432 (localhost): dial error: dial tcp 127.0.0.1:5432: connect: connection refused")
	hint := db.LocalUnreachableHint(err)
	if hint == "" {
		t.Fatal("expected local Postgres hint")
	}
	if !strings.Contains(hint, "make postgres") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestLocalUnreachableHintRemoteRefused(t *testing.T) {
	err := errors.New("dial tcp 203.0.113.10:5432: connect: connection refused")
	if hint := db.LocalUnreachableHint(err); hint != "" {
		t.Fatalf("unexpected hint for remote host: %q", hint)
	}
}

func TestLocalUnreachableHintOtherError(t *testing.T) {
	err := errors.New("password authentication failed for user \"one\"")
	if hint := db.LocalUnreachableHint(err); hint != "" {
		t.Fatalf("unexpected hint: %q", hint)
	}
}
