package git

import (
	"context"
	"strings"
)

// WorktreeInfo holds metadata about a single git worktree.
type WorktreeInfo struct {
	Path   string
	Commit string
	Branch string
	Bare   bool
}

// CreateWorktree adds a worktree at the given filesystem path, checking out
// the specified branch.
func (r *Repo) CreateWorktree(ctx context.Context, path, branch string) error {
	_, err := r.run(ctx, "worktree", "add", path, branch)
	return err
}

// CreateWorktreeNewBranch adds a worktree at worktreePath and simultaneously
// creates newBranch based on baseBranch.
func (r *Repo) CreateWorktreeNewBranch(ctx context.Context, worktreePath, newBranch, baseBranch string) error {
	_, err := r.run(ctx, "worktree", "add", "-b", newBranch, worktreePath, baseBranch)
	return err
}

// RemoveWorktree forcefully removes the worktree at the given path.
func (r *Repo) RemoveWorktree(ctx context.Context, path string) error {
	_, err := r.run(ctx, "worktree", "remove", path, "--force")
	return err
}

// ListWorktrees returns information about every worktree associated with this
// repository, parsed from `git worktree list --porcelain`.
func (r *Repo) ListWorktrees(ctx context.Context) ([]WorktreeInfo, error) {
	out, err := r.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	if out == "" {
		return nil, nil
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			// A new worktree block starts with "worktree <path>".
			// If we already accumulated one, save it first.
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{
				Path: strings.TrimPrefix(line, "worktree "),
			}

		case strings.HasPrefix(line, "HEAD "):
			current.Commit = strings.TrimPrefix(line, "HEAD ")

		case strings.HasPrefix(line, "branch "):
			// Branch is given as a full ref, e.g. "refs/heads/main".
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")

		case line == "bare":
			current.Bare = true

		case line == "detached":
			// Detached HEAD — branch stays empty.

		case line == "":
			// Blank line separates worktree blocks; handled by the
			// "worktree " prefix check above.
		}
	}

	// Don't forget the last entry.
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// PruneWorktrees removes stale worktree administrative data for worktrees
// whose directories have been deleted externally.
func (r *Repo) PruneWorktrees(ctx context.Context) error {
	_, err := r.run(ctx, "worktree", "prune")
	return err
}
