// Package gitremote seeds customer Git remotes for one/v1 (ADR-012).
// Day-to-day clone/pull stays in Control IDE; this package only does privileged init push.
package gitremote

import (
	"context"
	"errors"
)

var (
	// ErrAlreadyInitialized is returned when remote main already has a Majesta One baseline
	// and force was false.
	ErrAlreadyInitialized = errors.New("customer repository already initialized")
	// ErrNotConfigured is returned when no remote URL is set.
	ErrNotConfigured = errors.New("customer repository URL is not configured")
)

// Remote seeds one/v1 trees onto a Git host main branch.
type Remote interface {
	// SeedMain commits the contents of localDir and pushes to origin main.
	// Returns the commit SHA. If the remote already looks initialized and force is
	// false, returns ErrAlreadyInitialized.
	SeedMain(ctx context.Context, localDir string, force bool) (commitSHA string, err error)
}

// Auth is optional HTTP basic auth for HTTPS remotes (CodeCommit HTTPS Git
// credentials, GitHub PAT, GitLab token, etc.).
type Auth struct {
	Username string
	Password string
}
