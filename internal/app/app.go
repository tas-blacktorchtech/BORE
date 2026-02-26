package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"bore/internal/config"
	"bore/internal/db"
	"bore/internal/git"
	"bore/internal/logging"
	"bore/internal/process"
)

// App holds all application-wide state for bore.
// It is created once and shared with the TUI layer.
type App struct {
	cluster      *db.Cluster
	db           *db.DB
	config       *config.Config
	state        *config.State
	repo         *git.Repo
	logs         *logging.Manager
	runner       *process.Runner
	scheduler    *process.Scheduler
	boreDir      string
	statePath    string
	emitMu       sync.RWMutex
	eventEmitter func(event, data string)
	taskCancel   context.CancelFunc
	taskDone     chan struct{}
}

// Cluster returns the currently open cluster. Nil if none is open.
func (a *App) Cluster() *db.Cluster { return a.cluster }

// DB returns the database connection for the current cluster.
func (a *App) DB() *db.DB { return a.db }

// Config returns the loaded configuration for the current cluster.
func (a *App) Config() *config.Config { return a.config }

// State returns the lightweight UI state.
func (a *App) State() *config.State { return a.state }

// Repo returns the git repository for the current cluster.
func (a *App) Repo() *git.Repo { return a.repo }

// Logs returns the logging manager for the current cluster.
func (a *App) Logs() *logging.Manager { return a.logs }

// Runner returns the Claude CLI process runner.
func (a *App) Runner() *process.Runner { return a.runner }

// Scheduler returns the global worker concurrency scheduler.
func (a *App) Scheduler() *process.Scheduler { return a.scheduler }

// BoreDir returns the path to the .bore/ directory for the current cluster.
func (a *App) BoreDir() string { return a.boreDir }

// SetEventEmitter registers a callback that fires SSE events to connected
// browser clients. Call with event name and JSON data string.
func (a *App) SetEventEmitter(fn func(event, data string)) {
	a.emitMu.Lock()
	a.eventEmitter = fn
	a.emitMu.Unlock()
}

// Emit fires an SSE event if an emitter has been registered.
func (a *App) Emit(event, data string) {
	a.emitMu.RLock()
	fn := a.eventEmitter
	a.emitMu.RUnlock()
	if fn != nil {
		fn(event, data)
	}
}

// New creates a new App instance. It does NOT open a cluster yet.
func New() *App {
	return &App{}
}

// KnownClusters returns the known cluster paths from the global state file.
// Safe to call before any cluster is opened.
func (a *App) KnownClusters() []string {
	return loadGlobalState().KnownClusters
}

// LastCluster returns the last opened cluster path, or "" if none.
func (a *App) LastCluster() string {
	return loadGlobalState().LastCluster
}

// DeleteCluster removes a cluster by its repo path: deletes the DB record,
// removes the .bore/ directory, and removes it from the known clusters list.
// If the cluster is currently open, it is closed first.
func (a *App) DeleteCluster(ctx context.Context, repoPath string) error {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("app: delete cluster: resolve path: %w", err)
	}

	// If this is the currently open cluster, close it.
	if a.cluster != nil && a.cluster.RepoPath == absPath {
		if err := a.Close(); err != nil {
			return fmt.Errorf("app: delete cluster: close: %w", err)
		}
		a.cluster = nil
		a.db = nil
		a.repo = nil
	}

	// Open the cluster's DB to delete the record.
	boreDir := filepath.Join(absPath, ".bore")
	dbPath := filepath.Join(boreDir, "bore.db")
	if _, err := os.Stat(dbPath); err == nil {
		database, err := db.Open(dbPath)
		if err == nil {
			c, err := database.GetClusterByPath(ctx, absPath)
			if err == nil && c != nil {
				_ = database.DeleteCluster(ctx, c.ID)
			}
			database.Close()
		}
	}

	// Remove the .bore/ directory.
	if err := os.RemoveAll(boreDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("app: delete cluster: remove .bore: %w", err)
	}

	// Remove from known clusters list.
	if err := removeKnownCluster(absPath); err != nil {
		return fmt.Errorf("app: delete cluster: update known: %w", err)
	}

	return nil
}

// Close cleanly shuts down all resources.
func (a *App) Close() error {
	if a.taskCancel != nil {
		a.taskCancel()
		a.taskCancel = nil
	}
	// Wait for the task runner goroutine to exit before closing resources.
	if a.taskDone != nil {
		<-a.taskDone
		a.taskDone = nil
	}

	var errs []error

	if a.state != nil && a.statePath != "" {
		if err := config.SaveState(a.state, a.statePath); err != nil {
			errs = append(errs, fmt.Errorf("app: save state: %w", err))
		}
	}

	if a.logs != nil {
		if err := a.logs.Close(); err != nil {
			errs = append(errs, fmt.Errorf("app: close logs: %w", err))
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("app: close db: %w", err))
		}
	}

	return errors.Join(errs...)
}
