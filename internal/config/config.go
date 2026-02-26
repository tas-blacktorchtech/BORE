package config

import (
	"fmt"
	"strings"
)

// UIConfig holds user-interface settings.
type UIConfig struct {
	Theme string `json:"theme"`
}

// AgentsConfig holds agent orchestration settings.
type AgentsConfig struct {
	ClaudeCLIPath         string `json:"claude_cli_path"`
	DefaultModel          string `json:"default_model"`
	CommanderContextLimit int    `json:"commander_context_limit"`
	MaxTotalWorkers       int    `json:"max_total_workers"`
	MaxWorkersBasic       int    `json:"max_workers_basic"`
	MaxWorkersMedium      int    `json:"max_workers_medium"`
	MaxWorkersComplex     int    `json:"max_workers_complex"`
}

// GitConfig holds git integration settings.
type GitConfig struct {
	WorktreeStrategy string `json:"worktree_strategy"`
	ReviewRequired   bool   `json:"review_required"`
	AutoCommit       bool   `json:"auto_commit"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level      string `json:"level"`
	ToConsole  bool   `json:"to_console"`
	RotationMB int    `json:"rotation_mb"`
}

// Config is the top-level configuration for bore.
// Stored as .bore/config.json inside the target repository.
type Config struct {
	UI      UIConfig      `json:"ui"`
	Agents  AgentsConfig  `json:"agents"`
	Git     GitConfig     `json:"git"`
	Logging LoggingConfig `json:"logging"`
}

// DefaultConfig returns a Config populated with all default values.
func DefaultConfig() Config {
	return Config{
		UI: UIConfig{
			Theme: "navy_red_dark",
		},
		Agents: AgentsConfig{
			ClaudeCLIPath:         "claude",
			DefaultModel:          "",
			CommanderContextLimit: 5,
			MaxTotalWorkers:       6,
			MaxWorkersBasic:       1,
			MaxWorkersMedium:      2,
			MaxWorkersComplex:     4,
		},
		Git: GitConfig{
			WorktreeStrategy: "worktree",
			ReviewRequired:   true,
			AutoCommit:       false,
		},
		Logging: LoggingConfig{
			Level:      "info",
			ToConsole:  true,
			RotationMB: 10,
		},
	}
}

// Load reads a config from the JSON file at path and merges it with defaults
// so that any missing fields receive their default values. If the file does
// not exist, a fully-default Config is returned.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if err := loadJSON(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	EnsureDefaults(&cfg)

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes the config to path as indented JSON. Parent directories are
// created if they do not already exist.
func Save(cfg *Config, path string) error {
	if err := saveJSON(path, cfg, 0o644); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// isValidLogLevel reports whether s is an acceptable logging.level value.
func isValidLogLevel(s string) bool {
	switch s {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// Validate checks cfg for constraint violations and returns a combined error
// describing every problem found, or nil if the config is valid.
func Validate(cfg *Config) error {
	var errs []string

	if !isValidLogLevel(cfg.Logging.Level) {
		errs = append(errs, fmt.Sprintf("logging.level must be one of debug, info, warn, error; got %q", cfg.Logging.Level))
	}

	if cfg.Agents.MaxTotalWorkers < 1 {
		errs = append(errs, fmt.Sprintf("agents.max_total_workers must be >= 1; got %d", cfg.Agents.MaxTotalWorkers))
	}

	if cfg.Agents.MaxWorkersBasic < 1 {
		errs = append(errs, fmt.Sprintf("agents.max_workers_basic must be >= 1; got %d", cfg.Agents.MaxWorkersBasic))
	}

	if cfg.Agents.MaxWorkersMedium < 1 {
		errs = append(errs, fmt.Sprintf("agents.max_workers_medium must be >= 1; got %d", cfg.Agents.MaxWorkersMedium))
	}

	if cfg.Agents.MaxWorkersComplex < 1 {
		errs = append(errs, fmt.Sprintf("agents.max_workers_complex must be >= 1; got %d", cfg.Agents.MaxWorkersComplex))
	}

	if cfg.Agents.CommanderContextLimit < 0 {
		errs = append(errs, fmt.Sprintf("agents.commander_context_limit must be >= 0; got %d", cfg.Agents.CommanderContextLimit))
	}

	if cfg.Logging.RotationMB < 1 {
		errs = append(errs, fmt.Sprintf("logging.rotation_mb must be >= 1; got %d", cfg.Logging.RotationMB))
	}

	if cfg.Git.WorktreeStrategy != "worktree" {
		errs = append(errs, fmt.Sprintf("git.worktree_strategy must be \"worktree\" (V1 only supports worktree); got %q", cfg.Git.WorktreeStrategy))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config: %s", strings.Join(errs, "; "))
	}

	return nil
}

// EnsureDefaults fills in zero-value string fields in cfg with their default
// values. This is a public utility for manually constructed Config values.
// Numeric fields are intentionally left alone: a zero value may be the
// caller's explicit intent, and Load already unmarshals on top of
// DefaultConfig so missing JSON fields receive defaults automatically.
func EnsureDefaults(cfg *Config) {
	d := DefaultConfig()

	// UI
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = d.UI.Theme
	}

	// Agents — only patch string fields; numeric fields are left as-is.
	if cfg.Agents.ClaudeCLIPath == "" {
		cfg.Agents.ClaudeCLIPath = d.Agents.ClaudeCLIPath
	}
	// DefaultModel intentionally left alone — empty string is a valid value.

	// Git
	if cfg.Git.WorktreeStrategy == "" {
		cfg.Git.WorktreeStrategy = d.Git.WorktreeStrategy
	}

	// Logging
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = d.Logging.Level
	}
}
