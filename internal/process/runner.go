package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Runner executes Claude CLI as an external process.
type Runner struct {
	cliPath string // path to claude binary (default "claude")
	model   string // optional model override (pass as flag if non-empty)
}

// NewRunner creates a Runner. cliPath is the path/name of the claude binary.
// model is optional — if non-empty, it's passed as a flag.
func NewRunner(cliPath string, model string) *Runner {
	if cliPath == "" {
		cliPath = "claude"
	}
	return &Runner{
		cliPath: cliPath,
		model:   model,
	}
}

// RunResult holds the outcome of a CLI invocation.
type RunResult struct {
	Stdout    string // full stdout accumulated
	Stderr    string // full stderr accumulated
	ExitCode  int
	JSONBlock string // extracted last JSON block
	Err       error
}

// Run executes claude CLI with the given prompt piped via stdin.
// It runs in the specified workDir (for Workers this is the worktree directory).
// env is optional additional environment variables (key=value strings).
// onStdout and onStderr are called for each line of output as it arrives (for live streaming to TUI/logs).
// These callbacks may be nil.
// The function blocks until the process exits.
func (r *Runner) Run(ctx context.Context, workDir string, prompt string, env []string, onStdout func(line string), onStderr func(line string)) *RunResult {
	return r.RunWithSystem(ctx, workDir, "", prompt, env, onStdout, onStderr)
}

// RunWithSystem executes claude CLI with an explicit system prompt and a user
// prompt piped via stdin. systemPrompt may be empty, in which case no --system
// flag is passed.
func (r *Runner) RunWithSystem(ctx context.Context, workDir string, systemPrompt, userPrompt string, env []string, onStdout func(line string), onStderr func(line string)) *RunResult {
	args := []string{"-p", "--dangerously-skip-permissions"}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, r.cliPath, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(userPrompt)

	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return &RunResult{Err: fmt.Errorf("process: stdout pipe: %w", err)}
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return &RunResult{Err: fmt.Errorf("process: stderr pipe: %w", err)}
	}

	if err := cmd.Start(); err != nil {
		return &RunResult{Err: fmt.Errorf("process: start: %w", err)}
	}

	var stdoutBuf, stderrBuf strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 0, 1<<20), 10<<20)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line)
			stdoutBuf.WriteByte('\n')
			if onStdout != nil {
				onStdout(line)
			}
		}
		if err := scanner.Err(); err != nil {
			stdoutBuf.WriteString(fmt.Sprintf("[scanner error: %v]\n", err))
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 0, 1<<20), 10<<20)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteByte('\n')
			if onStderr != nil {
				onStderr(line)
			}
		}
		if err := scanner.Err(); err != nil {
			stderrBuf.WriteString(fmt.Sprintf("[scanner error: %v]\n", err))
		}
	}()

	wg.Wait()

	result := &RunResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if err := cmd.Wait(); err != nil {
		result.Err = fmt.Errorf("process: wait: %w", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}

	result.JSONBlock = extractLastJSON(result.Stdout)

	return result
}
