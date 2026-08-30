package gitremote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// GoGitRemote seeds an HTTPS (or file://) remote using pure-Go go-git.
type GoGitRemote struct {
	URL  string
	Auth *Auth
}

// NewGoGitRemote constructs a remote seeder. url must be non-empty.
func NewGoGitRemote(url string, auth *Auth) (*GoGitRemote, error) {
	if url == "" {
		return nil, ErrNotConfigured
	}
	return &GoGitRemote{URL: url, Auth: auth}, nil
}

func (r *GoGitRemote) authMethod() transport.AuthMethod {
	if r.Auth == nil || (r.Auth.Username == "" && r.Auth.Password == "") {
		return nil
	}
	user := r.Auth.Username
	if user == "" {
		user = "git"
	}
	return &http.BasicAuth{Username: user, Password: r.Auth.Password}
}

// SeedMain implements Remote.
func (r *GoGitRemote) SeedMain(ctx context.Context, localDir string, force bool) (string, error) {
	if r.URL == "" {
		return "", ErrNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	auth := r.authMethod()
	initialized, err := r.remoteInitialized(auth)
	if err != nil {
		// Unreachable / empty remote is OK — treat as not initialized.
		initialized = false
	}
	if initialized && !force {
		return "", ErrAlreadyInitialized
	}

	// Remove any nested .git from a previous attempt inside the export dir.
	_ = os.RemoveAll(filepath.Join(localDir, ".git"))

	repo, err := git.PlainInit(localDir, false)
	if err != nil {
		return "", fmt.Errorf("git init: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}
	hash, err := wt.Commit("Initialize one/v1 from install snapshot", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Majesta One Deploy",
			Email: "deploy@one.local",
			When:  time.Now().UTC(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.URL},
	})
	if err != nil {
		return "", fmt.Errorf("git remote: %w", err)
	}

	refSpec := config.RefSpec("+refs/heads/master:refs/heads/main")
	// PlainInit creates master by default in older go-git; newer may use main.
	head, err := repo.Head()
	if err == nil && head != nil {
		name := head.Name().Short()
		refSpec = config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/heads/main", name))
	}

	pushOpts := &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refSpec},
		Auth:       auth,
		Force:      force,
	}
	if err := repo.PushContext(ctx, pushOpts); err != nil {
		if err == git.NoErrAlreadyUpToDate {
			return hash.String(), nil
		}
		return "", fmt.Errorf("git push: %w", err)
	}
	return hash.String(), nil
}

func (r *GoGitRemote) remoteInitialized(auth transport.AuthMethod) (bool, error) {
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.URL},
	})
	refs, err := rem.List(&git.ListOptions{Auth: auth})
	if err != nil {
		return false, err
	}
	for _, ref := range refs {
		if ref.Name() == plumbing.NewBranchReferenceName("main") ||
			ref.Name() == plumbing.NewBranchReferenceName("master") {
			return !ref.Hash().IsZero(), nil
		}
	}
	return false, nil
}

// MemoryRemote is an in-process fake for tests (no network).
type MemoryRemote struct {
	URL         string
	Initialized bool
	LastDir     string
	ForceUsed   bool
	CommitSHA   string
	SeedErr     error
}

func (m *MemoryRemote) SeedMain(_ context.Context, localDir string, force bool) (string, error) {
	if m.SeedErr != nil {
		return "", m.SeedErr
	}
	if m.URL == "" {
		return "", ErrNotConfigured
	}
	if m.Initialized && !force {
		return "", ErrAlreadyInitialized
	}
	// Ensure one.yaml exists in the seeded tree.
	if _, err := os.Stat(filepath.Join(localDir, "one.yaml")); err != nil {
		return "", fmt.Errorf("seed tree missing one.yaml: %w", err)
	}
	m.LastDir = localDir
	m.ForceUsed = force
	m.Initialized = true
	if m.CommitSHA == "" {
		m.CommitSHA = "deadbeef"
	}
	return m.CommitSHA, nil
}
