package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runGit executes a standalone git command (not tied to a Repo) and returns
// the trimmed stdout. On failure the returned error includes stderr when
// available.
func runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Clone clones a remote repository from url into destPath.
// This is a standalone function that does not require an existing Repo.
func Clone(ctx context.Context, url, destPath string) error {
	_, err := runGit(ctx, "clone", url, destPath)
	return err
}

// IsGitRepo reports whether the directory at path is inside a git repository.
// It first checks for a .git entry, then falls back to asking git directly.
func IsGitRepo(ctx context.Context, path string) bool {
	// Fast path: check for .git directory or file.
	if info, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return info.IsDir() || info.Mode().IsRegular()
	}

	// Slow path: ask git.
	_, err := runGit(ctx, "-C", path, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// GetRemoteURL returns the URL of the "origin" remote for the repository at
// the given path.
func GetRemoteURL(ctx context.Context, path string) (string, error) {
	return runGit(ctx, "-C", path, "remote", "get-url", "origin")
}
