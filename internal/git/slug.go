package git

import (
	"fmt"
	"strings"
)

// Slugify converts an arbitrary string into a URL-safe slug suitable for use
// in git branch names.  It lowercases the input, replaces spaces and special
// characters with hyphens, collapses consecutive hyphens, trims leading and
// trailing hyphens, and caps the result at 50 characters.
//
// The output is restricted to ASCII letters and digits, ensuring that
// byte-length truncation is always safe.
func Slugify(s string) string {
	s = strings.ToLower(s)

	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			// Replace any non-ASCII-alphanumeric character with a hyphen.
			b.WriteByte('-')
		}
	}

	slug := b.String()

	// Collapse runs of hyphens into a single hyphen.
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	slug = strings.Trim(slug, "-")

	if slug == "" {
		return "untitled"
	}

	// Cap at 50 characters. Since the output is pure ASCII, byte length
	// is identical to character length.
	if len(slug) > 50 {
		slug = slug[:50]
		slug = strings.TrimRight(slug, "-")
	}

	return slug
}

// MakeExecBranch produces a deterministic branch name for an execution.
// Format: bore/{thread-slug}-{taskID}-{short-slug}
//
// short-slug is derived from the first ~6 words of taskTitle, slugified.
func MakeExecBranch(threadName string, taskID int64, taskTitle string) string {
	threadSlug := Slugify(threadName)

	// Build a short slug from the first ~6 words of the title.
	words := strings.Fields(taskTitle)
	if len(words) > 6 {
		words = words[:6]
	}
	shortSlug := Slugify(strings.Join(words, " "))

	return fmt.Sprintf("bore/%s-%d-%s", threadSlug, taskID, shortSlug)
}
