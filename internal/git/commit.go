package git

import (
	"context"
	"fmt"
)

// AddAll stages all changes (new, modified, deleted) in the working tree at dir.
func (r *Repo) AddAll(ctx context.Context, dir string) error {
	_, err := r.runInDir(ctx, dir, "add", "-A")
	return err
}

// Commit creates a commit in the working tree at dir with the given message.
func (r *Repo) Commit(ctx context.Context, dir, message string) error {
	_, err := r.runInDir(ctx, dir, "commit", "-m", message)
	return err
}

// Revert discards all changes (tracked and untracked) in the working tree
// at dir, restoring it to the state of the last commit. When force is false
// and the working tree has uncommitted modifications, Revert returns an error
// instead of silently discarding work.
func (r *Repo) Revert(ctx context.Context, dir string, force bool) error {
	if !force {
		has, err := r.HasChanges(ctx, dir)
		if err != nil {
			return fmt.Errorf("git revert: check changes: %w", err)
		}
		if has {
			return fmt.Errorf("git revert: working tree has changes, use force=true to discard")
		}
	}

	if _, err := r.runInDir(ctx, dir, "checkout", "--", "."); err != nil {
		return err
	}
	_, err := r.runInDir(ctx, dir, "clean", "-fd")
	return err
}

// GetCommitLog returns the most recent commits (one per line) from the
// working tree at dir.
func (r *Repo) GetCommitLog(ctx context.Context, dir string, count int) (string, error) {
	return r.runInDir(ctx, dir, "log", "--oneline", "-n", fmt.Sprintf("%d", count))
}

// MergeInto checks out targetBranch in the main repo and merges sourceBranch
// into it using --no-ff to preserve branch history.
func (r *Repo) MergeInto(ctx context.Context, targetBranch, sourceBranch string) error {
	if _, err := r.run(ctx, "checkout", targetBranch); err != nil {
		return fmt.Errorf("git: checkout %s: %w", targetBranch, err)
	}
	if _, err := r.run(ctx, "merge", "--no-ff", sourceBranch, "-m",
		fmt.Sprintf("bore-tui: merge %s into %s", sourceBranch, targetBranch)); err != nil {
		return fmt.Errorf("git: merge %s: %w", sourceBranch, err)
	}
	return nil
}
