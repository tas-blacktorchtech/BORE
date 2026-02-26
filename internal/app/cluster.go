package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"bore/internal/config"
	"bore/internal/db"
	"bore/internal/git"
	"bore/internal/logging"
	"bore/internal/process"
)

// InitCluster creates a new cluster from an existing repo path.
// It creates the .bore/ directory structure, initializes the DB,
// creates default config, and registers the cluster in the DB.
func (a *App) InitCluster(ctx context.Context, repoPath string) error {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("app: resolve repo path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("app: repo path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("app: repo path is not a directory: %s", absPath)
	}

	if !git.IsGitRepo(ctx, absPath) {
		return fmt.Errorf("app: not a git repository: %s", absPath)
	}

	boreDir := filepath.Join(absPath, ".bore")

	dirs := []string{
		boreDir,
		filepath.Join(boreDir, "logs"),
		filepath.Join(boreDir, "logs", "workers"),
		filepath.Join(boreDir, "runs"),
		filepath.Join(boreDir, "worktrees"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("app: create directory %s: %w", dir, err)
		}
	}

	// Open DB once for init.
	dbPath := filepath.Join(boreDir, "bore.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("app: init db: %w", err)
	}

	cfgPath := filepath.Join(boreDir, "config.json")
	defaultCfg := config.DefaultConfig()
	if err := config.Save(&defaultCfg, cfgPath); err != nil {
		database.Close()
		return fmt.Errorf("app: save default config: %w", err)
	}

	statePath := filepath.Join(boreDir, "state.json")
	defaultState := &config.State{
		SelectedItems: make(map[string]string),
	}
	if err := config.SaveState(defaultState, statePath); err != nil {
		database.Close()
		return fmt.Errorf("app: save default state: %w", err)
	}

	// Get the cluster name and remote URL.
	name := filepath.Base(absPath)
	remoteURL, _ := git.GetRemoteURL(ctx, absPath)

	// Create the cluster record in the same connection.
	_, err = database.CreateCluster(ctx, name, absPath, remoteURL)
	if err != nil {
		database.Close()
		return fmt.Errorf("app: create cluster record: %w", err)
	}
	database.Close()

	if err := ensureGitignore(absPath); err != nil {
		return fmt.Errorf("app: update gitignore: %w", err)
	}

	return a.OpenCluster(ctx, absPath)
}

// InitClusterFromClone clones a git repo and then initializes a cluster.
func (a *App) InitClusterFromClone(ctx context.Context, cloneURL string, destPath string) error {
	absPath, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("app: resolve dest path: %w", err)
	}

	if err := git.Clone(ctx, cloneURL, absPath); err != nil {
		return fmt.Errorf("app: clone: %w", err)
	}

	return a.InitCluster(ctx, absPath)
}

// OpenCluster opens an existing cluster from its repo path.
// It loads the DB, config, state, git repo, logging, and sets up
// the Runner and Scheduler.
func (a *App) OpenCluster(ctx context.Context, repoPath string) error {
	// Close any previously open cluster to prevent resource leaks.
	if a.db != nil || a.logs != nil {
		_ = a.Close()
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("app: resolve repo path: %w", err)
	}

	boreDir := filepath.Join(absPath, ".bore")
	if _, err := os.Stat(boreDir); err != nil {
		return fmt.Errorf("app: .bore directory not found: %w", err)
	}

	dbPath := filepath.Join(boreDir, "bore.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("app: open db: %w", err)
	}

	cfgPath := filepath.Join(boreDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		database.Close()
		return fmt.Errorf("app: load config: %w", err)
	}

	statePath := filepath.Join(boreDir, "state.json")
	state, err := config.LoadState(statePath)
	if err != nil {
		database.Close()
		return fmt.Errorf("app: load state: %w", err)
	}

	repo, err := git.NewRepo(absPath)
	if err != nil {
		database.Close()
		return fmt.Errorf("app: open repo: %w", err)
	}

	logs, err := logging.NewManager(boreDir, cfg.Logging.Level, cfg.Logging.RotationMB, cfg.Logging.ToConsole)
	if err != nil {
		database.Close()
		return fmt.Errorf("app: init logging: %w", err)
	}

	runner := process.NewRunner(cfg.Agents.ClaudeCLIPath, cfg.Agents.DefaultModel)
	scheduler := process.NewScheduler(cfg.Agents.MaxTotalWorkers)

	cluster, err := database.GetClusterByPath(ctx, absPath)
	if err != nil {
		logs.Close()
		database.Close()
		return fmt.Errorf("app: find cluster: %w", err)
	}

	a.cluster = cluster
	a.db = database
	a.config = cfg
	a.state = state
	a.repo = repo
	a.logs = logs
	a.runner = runner
	a.scheduler = scheduler
	a.boreDir = boreDir
	a.statePath = statePath

	if err := a.recoverInterrupted(ctx); err != nil {
		logs.System.Warn("app: crash recovery failed: %s", err.Error())
	}

	logs.System.Info("app: cluster opened: %s (id=%d)", cluster.Name, cluster.ID)

	_ = addKnownCluster(absPath) // best-effort, ignore error

	// Start background task runner with a long-lived context (NOT the HTTP
	// request context which dies when the response is sent).
	taskCtx, taskCancel := context.WithCancel(context.Background())
	a.taskCancel = taskCancel
	a.taskDone = make(chan struct{})
	StartTaskRunner(taskCtx, a)

	return nil
}

// ListRecentClusters returns clusters from the DB, most recent first.
func (a *App) ListRecentClusters(ctx context.Context) ([]db.Cluster, error) {
	if a.db == nil {
		return nil, fmt.Errorf("app: no database open")
	}
	clusters, err := a.db.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list clusters: %w", err)
	}
	return clusters, nil
}
