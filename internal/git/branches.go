package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// ListBranches returns the names of all local branches.
func (r *Repo) ListBranches(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "branch", "--list")
	if err != nil {
		return nil, err
	}

	if out == "" {
		return nil, nil
	}

	var branches []string
	for _, line := range strings.Split(out, "\n") {
		// Each line from `git branch` looks like "  name" or "* name".
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if name != "" {
			branches = append(branches, name)
		}
	}
	return branches, nil
}

// CurrentBranch returns the name of the currently checked-out branch.
// Returns "HEAD" when in detached-HEAD state.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	return r.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

// BranchExists reports whether a local branch with the given name exists.
func (r *Repo) BranchExists(ctx context.Context, name string) (bool, error) {
	_, err := r.run(ctx, "rev-parse", "--verify", "refs/heads/"+name)
	if err != nil {
		// If the branch does not exist git exits non-zero.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateBranch creates a new branch at the tip of baseBranch.
func (r *Repo) CreateBranch(ctx context.Context, name, baseBranch string) error {
	_, err := r.run(ctx, "branch", name, baseBranch)
	return err
}

// DeleteBranch force-deletes the named branch.
func (r *Repo) DeleteBranch(ctx context.Context, name string) error {
	_, err := r.run(ctx, "branch", "-D", name)
	return err
}
