package config

import (
	"fmt"
)

// State holds lightweight UI state that is persisted between sessions.
// Stored as .bore/state.json inside the target repository.
type State struct {
	LastClusterID   int               `json:"last_cluster_id"`
	LastClusterPath string            `json:"last_cluster_path"`
	SelectedItems   map[string]string `json:"selected_items"`
}

// defaultState returns a State with all fields initialized to safe defaults.
func defaultState() State {
	return State{
		SelectedItems: make(map[string]string),
	}
}

// LoadState reads a State from the JSON file at path. If the file does not
// exist, a default (empty) State is returned. Missing fields in the JSON are
// filled with defaults.
func LoadState(path string) (*State, error) {
	st := defaultState()

	if err := loadJSON(path, &st); err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}

	// Ensure the map is never nil even if the JSON had "selected_items": null.
	if st.SelectedItems == nil {
		st.SelectedItems = make(map[string]string)
	}

	return &st, nil
}

// SaveState writes the state to path as indented JSON. Parent directories are
// created if they do not already exist.
func SaveState(state *State, path string) error {
	// Guard against nil map so the JSON output always contains an object.
	if state.SelectedItems == nil {
		state.SelectedItems = make(map[string]string)
	}

	if err := saveJSON(path, state, 0o644); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	return nil
}
