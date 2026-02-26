package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Repo represents a git repository at a given path.
type Repo struct {
	Path string
}

// NewRepo creates a new Repo pointing at the given directory.
// It returns an error if the path does not exist.
func NewRepo(path string) (*Repo, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("git: repo path: %w", err)
	}
	return &Repo{Path: path}, nil
}

// run executes a git command in the repo's root directory and returns
// the trimmed stdout. On failure the returned error includes stderr.
func (r *Repo) run(ctx context.Context, args ...string) (string, error) {
	return r.runInDir(ctx, r.Path, args...)
}

// runInDir executes a git command with -C targeting the supplied directory.
// This is used for worktree-specific operations where the working directory
// differs from the main repo root.
func (r *Repo) runInDir(ctx context.Context, dir string, args ...string) (string, error) {
	cmdArgs := make([]string, 0, 2+len(args))
	cmdArgs = append(cmdArgs, "-C", dir)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderrStr)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}
