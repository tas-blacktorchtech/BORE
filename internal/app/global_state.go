package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type globalState struct {
	LastCluster   string   `json:"last_cluster"`
	KnownClusters []string `json:"known_clusters"`
}

func globalStateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bore")
}

func globalStatePath() string {
	return filepath.Join(globalStateDir(), "state.json")
}

// loadGlobalState reads the global state file and returns it.
// It never returns an error — on any failure it returns a zero-value globalState.
func loadGlobalState() globalState {
	data, err := os.ReadFile(globalStatePath())
	if err != nil {
		return globalState{}
	}
	var gs globalState
	if err := json.Unmarshal(data, &gs); err != nil {
		return globalState{}
	}
	return gs
}

// saveGlobalState creates ~/.bore/ if needed and writes the state as JSON.
func saveGlobalState(gs globalState) error {
	dir := globalStateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("app: global state: create dir: %w", err)
	}
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		return fmt.Errorf("app: global state: marshal: %w", err)
	}
	if err := os.WriteFile(globalStatePath(), data, 0o644); err != nil {
		return fmt.Errorf("app: global state: write: %w", err)
	}
	return nil
}

// removeKnownCluster removes repoPath from KnownClusters and saves.
// If repoPath was the LastCluster, LastCluster is cleared.
func removeKnownCluster(repoPath string) error {
	gs := loadGlobalState()
	if gs.LastCluster == repoPath {
		gs.LastCluster = ""
	}
	var updated []string
	for _, p := range gs.KnownClusters {
		if p != repoPath {
			updated = append(updated, p)
		}
	}
	gs.KnownClusters = updated
	return saveGlobalState(gs)
}

// addKnownCluster adds repoPath to KnownClusters (deduped, most recent first,
// capped at 20) and updates LastCluster, then saves.
func addKnownCluster(repoPath string) error {
	gs := loadGlobalState()
	gs.LastCluster = repoPath

	// Build a deduped list with repoPath at the front.
	seen := make(map[string]bool)
	updated := []string{repoPath}
	seen[repoPath] = true
	for _, p := range gs.KnownClusters {
		if !seen[p] {
			updated = append(updated, p)
			seen[p] = true
		}
		if len(updated) >= 20 {
			break
		}
	}
	gs.KnownClusters = updated

	return saveGlobalState(gs)
}
