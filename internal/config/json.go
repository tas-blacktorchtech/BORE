package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// loadJSON reads the file at path and unmarshals its JSON content into target.
// If the file does not exist, target is left unchanged and no error is returned.
func loadJSON(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return nil
}

// saveJSON marshals data as indented JSON and writes it to path. Parent
// directories are created if they do not already exist.
func saveJSON(path string, data interface{}, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	buf = append(buf, '\n')

	if err := os.WriteFile(path, buf, perm); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}
