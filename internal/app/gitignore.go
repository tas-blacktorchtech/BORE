package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureGitignore makes sure the repo's .gitignore contains `.bore/`.
func ensureGitignore(repoPath string) error {
	gitignorePath := filepath.Join(repoPath, ".gitignore")

	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("app: read .gitignore: %w", err)
	}

	content := string(data)

	// Normalize line endings for consistent processing.
	originalHasCRLF := strings.Contains(content, "\r\n")
	content = strings.ReplaceAll(content, "\r\n", "\n")

	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == ".bore/" {
			return nil
		}
	}

	var newContent string
	if len(content) == 0 {
		newContent = ".bore/\n"
	} else if strings.HasSuffix(content, "\n") {
		newContent = content + ".bore/\n"
	} else {
		newContent = content + "\n.bore/\n"
	}

	if originalHasCRLF {
		newContent = strings.ReplaceAll(newContent, "\n", "\r\n")
	}

	if err := os.WriteFile(gitignorePath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("app: write .gitignore: %w", err)
	}

	return nil
}
