package git

import (
	"context"
	"strings"
)

// Status returns the short-format status of the working tree at dir.
func (r *Repo) Status(ctx context.Context, dir string) (string, error) {
	return r.runInDir(ctx, dir, "status", "--short")
}

// Diff returns the unstaged diff for the working tree at dir.
func (r *Repo) Diff(ctx context.Context, dir string) (string, error) {
	return r.runInDir(ctx, dir, "diff")
}

// DiffStaged returns the staged (index) diff for the working tree at dir.
func (r *Repo) DiffStaged(ctx context.Context, dir string) (string, error) {
	return r.runInDir(ctx, dir, "diff", "--staged")
}

// DiffAll returns the combined diff for dir: unstaged changes, staged changes,
// and the content of any new untracked files (which git diff normally ignores).
func (r *Repo) DiffAll(ctx context.Context, dir string) (string, error) {
	unstaged, err := r.Diff(ctx, dir)
	if err != nil {
		return "", err
	}

	staged, err := r.DiffStaged(ctx, dir)
	if err != nil {
		return "", err
	}

	// Show new untracked files as diffs so reviewers can see their content.
	untracked, err := r.diffUntracked(ctx, dir)
	if err != nil {
		return "", err
	}

	var parts []string
	if unstaged != "" {
		parts = append(parts, unstaged)
	}
	if staged != "" {
		parts = append(parts, staged)
	}
	if untracked != "" {
		parts = append(parts, untracked)
	}

	return strings.Join(parts, "\n\n"), nil
}

// diffUntracked returns a diff-style representation of new untracked files.
// It stages them temporarily with --intent-to-add and diffs, then unstages.
func (r *Repo) diffUntracked(ctx context.Context, dir string) (string, error) {
	// List untracked files.
	out, err := r.runInDir(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil || out == "" {
		return "", err
	}

	// Stage intent-to-add so git diff can show them.
	if _, err := r.runInDir(ctx, dir, "add", "--intent-to-add", "--all"); err != nil {
		return "", err
	}

	diff, err := r.runInDir(ctx, dir, "diff")
	if err != nil {
		// Try to unstage even if diff failed.
		_, _ = r.runInDir(ctx, dir, "reset", "HEAD")
		return "", err
	}

	// Unstage the intent-to-add files.
	if _, err := r.runInDir(ctx, dir, "reset", "HEAD"); err != nil {
		return diff, nil // return what we got even if reset fails
	}

	return diff, nil
}

// HasChanges reports whether the working tree at dir has any uncommitted
// modifications (staged or unstaged, including untracked files).
func (r *Repo) HasChanges(ctx context.Context, dir string) (bool, error) {
	out, err := r.Status(ctx, dir)
	if err != nil {
		return false, err
	}
	return out != "", nil
}
