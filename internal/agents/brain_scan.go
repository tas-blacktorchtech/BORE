package agents

import (
	"fmt"
	"strings"
)

// BuildRepoBrainScanPrompt builds a prompt for Claude to scan a repository
// and generate an initial Commander brain document.
// repoPath is the absolute path to the repository root.
// dirListing is the output of listing top-level files/dirs.
// readmeContent is the content of README.md if it exists (empty string if not).
// keyFiles maps filename → content for key config files found (go.mod, package.json, etc.).
func BuildRepoBrainScanPrompt(repoPath, dirListing, readmeContent string, keyFiles map[string]string) string {
	var b strings.Builder

	b.WriteString("You are the Commander of a BORE system — a local, persistent, multi-agent engineering orchestration system.\n")
	b.WriteString("You have just been pointed at a new repository and need to build your working knowledge of it.\n")
	b.WriteString("Your job is to produce a concise Commander Brain document that you will use to plan engineering tasks on this repo.\n\n")

	fmt.Fprintf(&b, "## Repository Path\n\n%s\n\n", repoPath)

	b.WriteString("## Top-Level Directory Listing\n\n")
	b.WriteString(dirListing)
	b.WriteString("\n\n")

	if strings.TrimSpace(readmeContent) != "" {
		b.WriteString("## README.md\n\n")
		b.WriteString(readmeContent)
		b.WriteString("\n\n")
	}

	if len(keyFiles) > 0 {
		b.WriteString("## Key Config Files\n\n")
		for name, content := range keyFiles {
			fmt.Fprintf(&b, "### %s\n\n", name)
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}

	b.WriteString(`## Your Task

Write a Commander Brain document for this repository. It will be injected into your system prompt for every future task on this repo, so write it as a concise, practical reference for yourself.

Cover all of the following:
1. What this project is and what it does (1-2 sentences)
2. The tech stack and key dependencies
3. Important directories and their purposes
4. Coding conventions and patterns observed
5. Any obvious constraints or things to be careful about
6. Suggested crew types that would make sense for this project (e.g. "backend", "frontend", "infra", "testing")

Guidelines:
- Keep it between 200 and 400 words. Be concise but complete.
- Write in plain text (markdown headings and bullets are fine).
- Write it for an AI Commander who will use it to plan engineering tasks — not for a human reader.
- Do NOT produce JSON, code blocks, or any special formatting markers.
- Output ONLY the brain document text — no preamble, no "here is the document", just the content itself.
`)

	return b.String()
}
