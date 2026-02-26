package process

import "encoding/json"

// extractLastJSON scans the text backwards to find the last complete JSON object
// (delimited by `{...}`) or JSON array (delimited by `[...]`).
// It handles nested braces/brackets and string escaping.
// Each candidate is validated with json.Valid; if invalid, scanning continues
// backward to find the next closing delimiter.
// Returns the extracted JSON string, or empty string if none found.
func extractLastJSON(text string) string {
	n := len(text)

	// Search backward for each closing delimiter, try to extract a balanced
	// block, validate it, and return the first valid one found.
	for end := n - 1; end >= 0; end-- {
		closeChar := text[end]
		if closeChar != '}' && closeChar != ']' {
			continue
		}

		var openChar byte
		if closeChar == '}' {
			openChar = '{'
		} else {
			openChar = '['
		}

		// Walk backwards from end, tracking depth and string context.
		depth := 0
		inString := false
		for i := end; i >= 0; i-- {
			ch := text[i]

			if inString {
				if ch == '"' && !isEscaped(text, i) {
					inString = false
				}
				continue
			}

			if ch == '"' && !isEscaped(text, i) {
				inString = true
				continue
			}

			if ch == closeChar {
				depth++
			} else if ch == openChar {
				depth--
			}

			if depth == 0 {
				candidate := text[i : end+1]
				if json.Valid([]byte(candidate)) {
					return candidate
				}
				// Invalid JSON — break inner loop and try the next
				// closing delimiter further back.
				break
			}
		}
	}

	return ""
}

// isEscaped reports whether the character at position pos is preceded by an
// odd number of backslashes (and thus escaped within a JSON string).
func isEscaped(text string, pos int) bool {
	count := 0
	for i := pos - 1; i >= 0 && text[i] == '\\'; i-- {
		count++
	}
	return count%2 != 0
}
