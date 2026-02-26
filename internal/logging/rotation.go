package logging

import (
	"fmt"
	"os"
	"strconv"
)

// maxBackups is the number of rotated copies to keep (file.log.1 … file.log.5).
const maxBackups = 5

// rotate performs a simple size-based log rotation for the file at filePath.
//
// The algorithm shifts existing backups upward by one:
//
//	.log.5 is deleted (if it exists)
//	.log.4 → .log.5
//	.log.3 → .log.4
//	.log.2 → .log.3
//	.log.1 → .log.2
//	.log   → .log.1
//
// After the shift the caller is responsible for opening a fresh .log file.
func rotate(filePath string) error {
	// Remove the oldest backup if it exists.
	oldest := filePath + "." + strconv.Itoa(maxBackups)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("logging: rotate remove %s: %w", oldest, err)
	}

	// Shift .log.(n-1) → .log.n, working from the highest index downward.
	for i := maxBackups - 1; i >= 1; i-- {
		src := filePath + "." + strconv.Itoa(i)
		dst := filePath + "." + strconv.Itoa(i+1)
		if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("logging: rotate rename %s -> %s: %w", src, dst, err)
		}
	}

	// Move the current log file to .log.1.
	dst := filePath + ".1"
	if err := os.Rename(filePath, dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("logging: rotate rename %s -> %s: %w", filePath, dst, err)
	}

	return nil
}
