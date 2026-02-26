package agents

import (
	"bore/internal/db"
	"fmt"
	"strings"
)

// BossContext holds dynamic data for a Boss prompt.
type BossContext struct {
	Crew         *db.Crew   // crew assigned at the execution level by Commander (may be nil)
	AllCrews     []db.Crew  // all crews defined in the cluster
	Brief        ExecutionBrief
	TaskPrompt   string
	Mode         string // "just_get_it_done" or "alert_with_issues"
	WorkerBudget int
}

// BuildBossSystemPrompt returns the Boss's system prompt with injected context.
func BuildBossSystemPrompt(ctx BossContext) string {
	var b strings.Builder

	b.WriteString(`You are **Boss**, a manager-only subagent for bore.

You have:
- The user task prompt
- The selected crew objective and constraints
- The chosen execution mode
- The worker budget and global worker cap rules (you will be told your budget)

Your responsibilities:
1) Create a step-by-step execution plan
2) Spawn Workers with narrow roles (do NOT do the work yourself)
3) Collect worker outputs and verify they meet success criteria
4) Decide if more workers are needed, staying within budget
5) Produce a final summary and lessons

Constraints:
- You must not edit files or run commands yourself.
- You must delegate all code/command work to Workers.
- If mode == "alert_with_issues", stop and ask for user help when blocked.
- You must output structured JSON only.
`)

	writeBossContextSection(&b, ctx)
	writeBossOutputFormats(&b)

	return b.String()
}

// BuildBossPlanPrompt returns the user message asking Boss to create a plan.
func BuildBossPlanPrompt(ctx BossContext) string {
	var b strings.Builder

	b.WriteString("## Task\n\n")
	b.WriteString(ctx.TaskPrompt)
	b.WriteString("\n\n")

	b.WriteString("## Execution Brief\n\n")
	fmt.Fprintf(&b, "- **Task title**: %s\n", ctx.Brief.TaskTitle)
	fmt.Fprintf(&b, "- **Base branch**: %s\n", ctx.Brief.BaseBranch)
	fmt.Fprintf(&b, "- **Thread**: %s\n", ctx.Brief.Thread)

	if len(ctx.Brief.Scope) > 0 {
		b.WriteString("- **Scope**:\n")
		for _, s := range ctx.Brief.Scope {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}

	if len(ctx.Brief.NotInScope) > 0 {
		b.WriteString("- **Not in scope**:\n")
		for _, s := range ctx.Brief.NotInScope {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}

	if len(ctx.Brief.SuccessCriteria) > 0 {
		b.WriteString("- **Success criteria**:\n")
		for _, s := range ctx.Brief.SuccessCriteria {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}

	b.WriteString("\n")

	b.WriteString(`## Instructions

Analyze the task and execution brief above. Create a step-by-step plan, identifying which workers you will need to spawn. Each step should map to a worker with a narrow role.

Respond with ONLY the following JSON (no markdown fences, no extra text):

{
  "type": "boss_plan",
  "steps": [
    {
      "id": "step1",
      "title": "Short step title",
      "detail": "What this step accomplishes",
      "worker_role": "Role name for the worker"
    }
  ],
  "validation": ["How to validate the overall result"],
  "estimated_files": ["files/likely/to/be/touched.go"],
  "needs_workers": [
    {
      "role": "Worker role name",
      "goal": "Specific goal for this worker",
      "crew": "team name from Available Teams, or empty string if none applies",
      "files_or_paths": ["target/files/or/dirs"],
      "commands": ["allowed commands to run"],
      "success_criteria": ["How to know this worker succeeded"]
    }
  ]
}
`)

	return b.String()
}

// BuildBossSummaryPrompt returns the user message asking Boss for a final summary.
// workerResults are the collected worker outputs from this execution.
func BuildBossSummaryPrompt(workerResults []WorkerResult) string {
	var b strings.Builder

	b.WriteString("## Worker Results\n\n")

	if len(workerResults) == 0 {
		b.WriteString("No worker results collected.\n\n")
	} else {
		for i, wr := range workerResults {
			fmt.Fprintf(&b, "### Worker %d - %s\n\n", i+1, wr.Outcome)
			if wr.Summary != "" {
				fmt.Fprintf(&b, "%s\n\n", wr.Summary)
			}
			if len(wr.FilesChanged) > 0 {
				b.WriteString("**Files changed**:\n")
				for _, f := range wr.FilesChanged {
					fmt.Fprintf(&b, "- %s\n", f)
				}
				b.WriteByte('\n')
			}
			if len(wr.CommandsRun) > 0 {
				b.WriteString("**Commands run**:\n")
				for _, c := range wr.CommandsRun {
					fmt.Fprintf(&b, "- `%s`\n", c)
				}
				b.WriteByte('\n')
			}
			if len(wr.ValidationResults) > 0 {
				b.WriteString("**Validation results**:\n")
				for _, v := range wr.ValidationResults {
					fmt.Fprintf(&b, "- %s\n", v)
				}
				b.WriteByte('\n')
			}
			if len(wr.Blockers) > 0 {
				b.WriteString("**Blockers**:\n")
				for _, bl := range wr.Blockers {
					fmt.Fprintf(&b, "- %s\n", bl)
				}
				b.WriteByte('\n')
			}
		}
	}

	b.WriteString(`## Instructions

Review all worker results above. Produce a final summary of the execution including the overall outcome, what changed, validation results, any risks or followups, and lessons learned.

Respond with ONLY the following JSON (no markdown fences, no extra text):

{
  "type": "boss_summary",
  "outcome": "success | partial | failed",
  "what_changed": ["Summary of each meaningful change"],
  "files_touched": ["all/files/modified.go"],
  "commands_run": ["all commands that were run"],
  "validation_results": ["Result of each validation step"],
  "risks_or_followups": ["Any remaining risks or follow-up tasks"],
  "lessons": [
    {
      "lesson_type": "error | pattern | warning | note",
      "content": "What was learned"
    }
  ]
}
`)

	return b.String()
}

func writeBossContextSection(b *strings.Builder, ctx BossContext) {
	b.WriteString("\n## Execution Context\n\n")
	fmt.Fprintf(b, "- **Execution mode**: %s\n", ctx.Mode)
	fmt.Fprintf(b, "- **Worker budget**: %d\n", ctx.WorkerBudget)

	if ctx.Crew != nil {
		fmt.Fprintf(b, "- **Assigned crew**: %s\n", ctx.Crew.Name)
	} else {
		b.WriteString("- **Assigned crew**: none\n")
	}

	if len(ctx.Brief.SuccessCriteria) > 0 {
		b.WriteString("- **Success criteria**:\n")
		for _, sc := range ctx.Brief.SuccessCriteria {
			fmt.Fprintf(b, "  - %s\n", sc)
		}
	}

	if len(ctx.Brief.KeyRisks) > 0 {
		b.WriteString("- **Key risks**:\n")
		for _, r := range ctx.Brief.KeyRisks {
			fmt.Fprintf(b, "  - %s\n", r)
		}
	}

	// Show all available crews so Boss can assign the right team to each worker.
	b.WriteString("\n## Available Teams\n\n")
	if len(ctx.AllCrews) == 0 {
		b.WriteString("No teams defined. Workers will run without team constraints.\n")
	} else {
		b.WriteString("These teams are available. When spawning workers, assign each worker to the most appropriate team by setting the `crew` field in `needs_workers`. Use the team name exactly as shown.\n\n")
		for _, c := range ctx.AllCrews {
			fmt.Fprintf(b, "### %s\n", c.Name)
			fmt.Fprintf(b, "- **Objective**: %s\n", c.Objective)
			if c.Constraints != "" {
				fmt.Fprintf(b, "- **Constraints**: %s\n", c.Constraints)
			}
			if c.AllowedCommands != "" {
				fmt.Fprintf(b, "- **Allowed commands**: %s\n", c.AllowedCommands)
			}
			if c.OwnershipPaths != "" {
				fmt.Fprintf(b, "- **Ownership paths**: %s\n", c.OwnershipPaths)
			}
			b.WriteByte('\n')
		}
	}
}

func writeBossOutputFormats(b *strings.Builder) {
	b.WriteString(`
## Output Format Rules

You will be asked to produce one of three output types per message. Each request will specify which format to use.
Always respond with ONLY the JSON object requested -- no markdown fences, no commentary, no extra text.

The three formats are:

1. **Boss Plan** (type: "boss_plan") -- your initial step-by-step execution plan with worker needs
2. **Spawn Workers** (type: "spawn_workers") -- requesting additional workers mid-execution
3. **Boss Summary** (type: "boss_summary") -- your final summary after all workers complete
`)
}
