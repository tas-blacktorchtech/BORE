package logging

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Manager is the central owner of every log category used by bore.
// It creates the required directory structure under boreDir and provides
// convenience methods for obtaining per-worker loggers and writing
// immutable run artifacts.
type Manager struct {
	mu sync.Mutex

	// System is the rolling log for startup, config, db, git and crash events.
	System *Logger

	// Commander is the rolling log for intake decisions, review options and
	// planning outputs.
	Commander *Logger

	// workers tracks loggers opened via WorkerLogger so they can all be
	// closed from Close().
	workers map[int64]*Logger

	logDir     string
	workersDir string
	runsDir    string
	level      string
	rotationMB int
	toConsole  bool
}

// NewManager initialises the logging directory tree under boreDir and opens
// the System and Commander loggers. The expected layout:
//
//	<boreDir>/logs/system.log
//	<boreDir>/logs/commander.log
//	<boreDir>/logs/workers/          (created now, populated on demand)
//	<boreDir>/runs/                  (created now, populated on demand)
func NewManager(boreDir string, level string, rotationMB int, toConsole bool) (*Manager, error) {
	logDir := filepath.Join(boreDir, "logs")
	workersDir := filepath.Join(logDir, "workers")
	runsDir := filepath.Join(boreDir, "runs")

	for _, dir := range []string{logDir, workersDir, runsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("logging: mkdir %s: %w", dir, err)
		}
	}

	sysLog, err := NewLogger(filepath.Join(logDir, "system.log"), level, rotationMB, toConsole)
	if err != nil {
		return nil, fmt.Errorf("logging: system logger: %w", err)
	}

	cmdLog, err := NewLogger(filepath.Join(logDir, "commander.log"), level, rotationMB, toConsole)
	if err != nil {
		sysLog.Close()
		return nil, fmt.Errorf("logging: commander logger: %w", err)
	}

	return &Manager{
		System:     sysLog,
		Commander:  cmdLog,
		workers:    make(map[int64]*Logger),
		logDir:     logDir,
		workersDir: workersDir,
		runsDir:    runsDir,
		level:      level,
		rotationMB: rotationMB,
		toConsole:  toConsole,
	}, nil
}

// WorkerLogger returns a logger for the given agent run ID. If a logger for
// that ID has already been opened it is returned directly; otherwise a new
// one is created at .bore/logs/workers/worker-{agentRunID}.log.
func (m *Manager) WorkerLogger(agentRunID int64) (*Logger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if l, ok := m.workers[agentRunID]; ok {
		return l, nil
	}

	name := "worker-" + strconv.FormatInt(agentRunID, 10) + ".log"
	path := filepath.Join(m.workersDir, name)

	l, err := NewLogger(path, m.level, m.rotationMB, m.toConsole)
	if err != nil {
		return nil, fmt.Errorf("logging: worker logger %d: %w", agentRunID, err)
	}

	m.workers[agentRunID] = l
	return l, nil
}

// EnsureRunDir creates the directory .bore/runs/{executionID}/ if it does
// not already exist and returns its absolute path.
func (m *Manager) EnsureRunDir(executionID int64) (string, error) {
	dir := filepath.Join(m.runsDir, strconv.FormatInt(executionID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("logging: run dir %d: %w", executionID, err)
	}
	return dir, nil
}

// WriteRunArtifact writes data to .bore/runs/{executionID}/{filename},
// creating the run directory if necessary. The filename is sanitized to
// prevent path traversal.
func (m *Manager) WriteRunArtifact(executionID int64, filename string, data []byte) error {
	filename = filepath.Base(filename)
	if filename == "." || filename == ".." {
		return fmt.Errorf("logging: invalid artifact filename")
	}

	dir, err := m.EnsureRunDir(executionID)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("logging: write artifact %s: %w", path, err)
	}
	return nil
}

// Close closes the System logger, Commander logger, and every worker logger
// that has been opened during the lifetime of this Manager. Pointers are
// copied under the lock and the lock is released before calling Close on
// each logger to avoid lock-ordering deadlocks.
func (m *Manager) Close() error {
	m.mu.Lock()
	sys := m.System
	cmd := m.Commander
	workers := make(map[int64]*Logger, len(m.workers))
	for k, v := range m.workers {
		workers[k] = v
	}
	m.workers = nil
	m.mu.Unlock()

	var errs []error
	if sys != nil {
		if err := sys.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if cmd != nil {
		if err := cmd.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, w := range workers {
		if err := w.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
