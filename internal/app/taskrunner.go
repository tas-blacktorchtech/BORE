package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bore/internal/agents"
	"bore/internal/db"
	"bore/internal/git"
)

// clarificationQuestion mirrors the JSON the Commander emits for clarifications.
type clarificationQuestion struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Why      string `json:"why"`
}

// parseClarificationQuestions extracts questions from the Commander's clarification JSON.
// Returns nil (not an error) if the JSON is malformed or empty.
func parseClarificationQuestions(jsonStr string) []clarificationQuestion {
	var resp struct {
		Questions []clarificationQuestion `json:"questions"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil
	}
	return resp.Questions
}

// StartTaskRunner launches the background task processing loop. It polls for
// pending tasks and runs the full Commander → Boss → Workers pipeline. It
// returns immediately and runs until ctx is cancelled.
func StartTaskRunner(ctx context.Context, a *App) {
	log.Printf("[taskrunner] started for cluster %q (id=%d)", a.cluster.Name, a.cluster.ID)
	go func() {
		defer func() {
			if a.taskDone != nil {
				close(a.taskDone)
			}
		}()
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		// Track tasks currently in-flight to avoid double-processing.
		var (
			mu       sync.Mutex
			inFlight = make(map[int64]bool)
		)

		for {
			select {
			case <-ctx.Done():
				log.Printf("[taskrunner] stopping (context cancelled)")
				return
			case <-ticker.C:
				tasks, err := a.db.ListTasksByStatus(ctx, a.cluster.ID, db.StatusPending)
				if err != nil {
					log.Printf("[taskrunner] poll error: %v", err)
					continue
				}
				for _, task := range tasks {
					mu.Lock()
					if inFlight[task.ID] {
						mu.Unlock()
						continue
					}
					inFlight[task.ID] = true
					mu.Unlock()

					log.Printf("[taskrunner] picked up task %d: %q", task.ID, task.Title)
					go func() {
						defer func() {
							mu.Lock()
							delete(inFlight, task.ID)
							mu.Unlock()
						}()
						if err := processTask(ctx, a, task); err != nil {
							log.Printf("[taskrunner] task %d FAILED: %v", task.ID, err)
						}
					}()
				}
			}
		}
	}()
}

// processTask runs the pipeline for a single task.
//
// Commander flow (interactive, two-phase):
//  1. Fresh task → run clarifications. If questions generated, pause at "review" for user.
//     If no questions, proceed immediately to options.
//  2. User answers clarifications (or they had none) → run options → pause at "review" for user.
//  3. User selects an option → run execution brief + boss + workers.
func processTask(ctx context.Context, a *App, task db.Task) error {
	logf := func(format string, args ...any) {
		log.Printf("[task:%d] "+format, append([]any{task.ID}, args...)...)
	}

	reviews, err := a.db.GetTaskReviews(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("taskrunner: get reviews: %w", err)
	}

	var (
		hasClarification    bool
		clarificationJSON   string
		hasClarAnswers      bool
		clarAnswersJSON     string
		hasOptions          bool
		hasSelection        bool
		selectedOptionID    string
	)
	for _, r := range reviews {
		switch r.Phase {
		case db.PhaseClarification:
			hasClarification = true
			clarificationJSON = r.Content
		case db.PhaseClarificationAnswers:
			hasClarAnswers = true
			clarAnswersJSON = r.Content
		case db.PhaseOptions:
			hasOptions = true
		case db.PhaseSelection:
			hasSelection = true
			selectedOptionID = r.Content
		}
	}

	// --- Phase 3: user selected an option → execute ---
	if hasSelection {
		logf("=== RESUMING TASK: %q (user selected option %q) ===", task.Title, selectedOptionID)
		return executeTask(ctx, a, task, reviews, selectedOptionID, logf)
	}

	// --- Options exist but no selection — shouldn't be pending. Fix status. ---
	if hasOptions {
		logf("task has options but no selection — setting back to review")
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusReview)
		a.Emit("tasks_updated", "{}")
		return nil
	}

	// --- Phase 2: user answered clarifications → generate options ---
	if hasClarAnswers {
		logf("=== GENERATING OPTIONS for task %q ===", task.Title)
		return runCommanderOptions(ctx, a, task, clarAnswersJSON, logf)
	}

	// --- Clarifications exist but no answers yet ---
	if hasClarification {
		questions := parseClarificationQuestions(clarificationJSON)
		if len(questions) == 0 {
			// Commander asked no questions — proceed directly to options.
			logf("task has clarifications (no questions) but no options — proceeding to options")
			return runCommanderOptions(ctx, a, task, "", logf)
		}
		// Questions waiting for user — fix status back to review.
		logf("task has clarification questions but no answers — setting back to review")
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusReview)
		a.Emit("tasks_updated", "{}")
		return nil
	}

	// --- Phase 1: fresh task → run clarifications ---
	logf("=== STARTING TASK: %q ===", task.Title)
	return runCommanderClarifications(ctx, a, task, logf)
}

// buildCommanderContext gathers all data the Commander needs for its system prompt.
func buildCommanderContext(ctx context.Context, a *App) agents.CommanderContext {
	brain, _ := a.db.GetAllMemory(ctx, a.cluster.ID)
	crews, _ := a.db.ListCrews(ctx, a.cluster.ID)
	threads, _ := a.db.ListThreads(ctx, a.cluster.ID)
	history, _ := a.db.ListTaskHistories(ctx, a.cluster.ID)
	lessons, _ := a.db.ListAllLessons(ctx, a.cluster.ID)
	pastRuns, _ := a.db.ListRecentAgentRuns(ctx, a.cluster.ID, 20)
	return agents.CommanderContext{
		Brain:       brain,
		Crews:       crews,
		Threads:     threads,
		TaskHistory: history,
		Lessons:     lessons,
		PastRuns:    pastRuns,
	}
}

// runCommanderClarifications is Phase 1 of the Commander flow.
// It immediately sets the task to "review" so the UI reflects progress, then
// asks the Commander for clarifying questions. If no questions are needed it
// proceeds directly to runCommanderOptions; otherwise it pauses so the user
// can answer in the UI.
func runCommanderClarifications(ctx context.Context, a *App, task db.Task, logf func(string, ...any)) error {
	logf("phase: commander clarifications")

	// Move to "review" right away — the UI can now show "Commander is analyzing..."
	// instead of the generic "Waiting for Commander to pick up this task...".
	if err := a.db.UpdateTaskStatus(ctx, task.ID, db.StatusReview); err != nil {
		return fmt.Errorf("taskrunner: set review for clarifications: %w", err)
	}
	a.Emit("tasks_updated", "{}")

	sysPrompt := agents.BuildCommanderSystemPrompt(buildCommanderContext(ctx, a))

	logf("commander: asking for clarifications")
	clarPrompt := agents.BuildClarificationPrompt(task.Prompt)
	clarResult := a.runner.RunWithSystem(ctx, a.repo.Path, sysPrompt, clarPrompt, nil,
		func(line string) { logf("commander stdout: %s", line) },
		func(line string) { logf("commander stderr: %s", line) },
	)
	if clarResult.Err != nil {
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		return fmt.Errorf("taskrunner: clarification run: %w", clarResult.Err)
	}

	clarJSON := clarResult.JSONBlock
	if clarJSON == "" {
		clarJSON = `{"type":"clarifications","questions":[]}`
	}
	if _, err := a.db.CreateTaskReview(ctx, task.ID, db.PhaseClarification, clarJSON); err != nil {
		logf("WARN: save clarification review: %v", err)
	}
	a.Emit("tasks_updated", "{}")

	questions := parseClarificationQuestions(clarJSON)
	if len(questions) == 0 {
		// No clarification questions needed — proceed directly to options.
		logf("commander: no clarifications needed, proceeding to options")
		return runCommanderOptions(ctx, a, task, "", logf)
	}

	logf("commander: %d clarification question(s) — waiting for user", len(questions))
	// Status is already "review". The frontend will display the questions.
	// The user submits answers via POST /api/tasks/{id}/clarifications,
	// which sets status back to "pending" so the task runner picks it up again.
	return nil
}

// runCommanderOptions is Phase 2 of the Commander flow.
// clarAnswersJSON is the JSON-encoded map[string]string of user answers (may be empty).
// It generates 2-3 execution options and pauses at "review" for the user to pick one.
func runCommanderOptions(ctx context.Context, a *App, task db.Task, clarAnswersJSON string, logf func(string, ...any)) error {
	logf("phase: commander options")

	// Ensure status is "review" (it may be "pending" if user just submitted answers).
	if err := a.db.UpdateTaskStatus(ctx, task.ID, db.StatusReview); err != nil {
		return fmt.Errorf("taskrunner: set review for options: %w", err)
	}
	a.Emit("tasks_updated", "{}")

	// Parse clarification answers if any.
	var clarAnswers map[string]string
	if clarAnswersJSON != "" {
		if err := json.Unmarshal([]byte(clarAnswersJSON), &clarAnswers); err != nil {
			logf("WARN: parse clarification answers: %v", err)
		}
	}

	sysPrompt := agents.BuildCommanderSystemPrompt(buildCommanderContext(ctx, a))

	logf("commander: asking for options")
	optPrompt := agents.BuildOptionsPrompt(task.Prompt, clarAnswers)
	optResult := a.runner.RunWithSystem(ctx, a.repo.Path, sysPrompt, optPrompt, nil,
		func(line string) { logf("commander stdout: %s", line) },
		func(line string) { logf("commander stderr: %s", line) },
	)
	if optResult.Err != nil {
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		return fmt.Errorf("taskrunner: options run: %w", optResult.Err)
	}

	optJSON := optResult.JSONBlock
	if optJSON == "" {
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		return fmt.Errorf("taskrunner: commander returned no JSON for options")
	}
	if _, err := a.db.CreateTaskReview(ctx, task.ID, db.PhaseOptions, optJSON); err != nil {
		logf("WARN: save options review: %v", err)
	}
	a.Emit("tasks_updated", "{}")

	logf("commander options saved — waiting for user to pick one")
	// Status is already "review". The user picks via POST /api/tasks/{id}/review,
	// which sets status back to "pending" with a "selection" review entry.
	return nil
}

// executeTask runs the brief + execution phases after the user has selected an option.
func executeTask(ctx context.Context, a *App, task db.Task, reviews []db.TaskReview, selectedOptionID string, logf func(string, ...any)) error {
	// Build Commander context for the brief.
	cmdCtx := buildCommanderContext(ctx, a)
	sysPrompt := agents.BuildCommanderSystemPrompt(cmdCtx)

	// Determine base branch.
	baseBranch, err := a.repo.CurrentBranch(ctx)
	if err != nil || baseBranch == "" || baseBranch == "HEAD" {
		baseBranch = "main"
		logf("WARN: could not determine current branch, using %q", baseBranch)
	}

	// Step 3: execution brief.
	logf("commander: generating execution brief (base=%q, option=%q)", baseBranch, selectedOptionID)
	if err := a.db.UpdateTaskStatus(ctx, task.ID, db.StatusRunning); err != nil {
		logf("mark running failed: %v", err)
	}
	a.Emit("tasks_updated", "{}")

	briefPrompt := agents.BuildExecutionBriefPrompt(task.Prompt, selectedOptionID, baseBranch)
	briefResult := a.runner.RunWithSystem(ctx, a.repo.Path, sysPrompt, briefPrompt, nil,
		func(line string) { logf("commander stdout: %s", line) },
		func(line string) { logf("commander stderr: %s", line) },
	)
	if briefResult.Err != nil {
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		return fmt.Errorf("taskrunner: brief run: %w", briefResult.Err)
	}
	logf("commander brief raw JSON: %q", briefResult.JSONBlock)

	if briefResult.JSONBlock == "" {
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		return fmt.Errorf("taskrunner: commander returned no JSON for brief")
	}
	parsed, err := agents.ParseResponse(briefResult.JSONBlock)
	if err != nil {
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		return fmt.Errorf("taskrunner: parse brief: %w", err)
	}
	brief, ok := parsed.(agents.ExecutionBrief)
	if !ok {
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		return fmt.Errorf("taskrunner: unexpected brief type %T", parsed)
	}
	if brief.BaseBranch == "" {
		brief.BaseBranch = baseBranch
	}
	logf("commander produced brief: branch=%q, crew=%q, workers=%d",
		brief.BaseBranch, brief.Crew, brief.WorkerBudget)

	// --- Create Execution ---
	logf("creating execution + worktree")
	exec, worktreePath, err := createExecution(ctx, a, task, brief, logf)
	if err != nil {
		logf("create execution failed: %v", err)
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		a.Emit("executions_updated", "{}")
		return fmt.Errorf("taskrunner: create execution: %w", err)
	}
	logf("execution %d created, worktree: %s", exec.ID, worktreePath)
	a.Emit("executions_updated", "{}")

	// --- Boss + Workers Phase ---
	logf("phase: boss planning + workers")
	if err := a.db.SetExecutionStarted(ctx, exec.ID); err != nil {
		logf("set started failed: %v", err)
	}
	if err := a.db.UpdateTaskStatus(ctx, task.ID, db.StatusRunning); err != nil {
		logf("mark running failed: %v", err)
	}
	a.Emit("tasks_updated", "{}")
	a.Emit("executions_updated", "{}")

	outcome, bossSummary, err := runBossWorkersPhase(ctx, a, task, exec, brief, worktreePath, logf)
	if err != nil {
		logf("boss/workers phase error: %v", err)
		_ = a.db.SetExecutionFinished(ctx, exec.ID, db.StatusFailed)
		_ = a.db.UpdateTaskStatus(ctx, task.ID, db.StatusFailed)
		a.Emit("tasks_updated", "{}")
		a.Emit("executions_updated", "{}")
		return fmt.Errorf("taskrunner: boss/workers: %w", err)
	}

	// Write completion record to .bore/runs/task-{id}.md for human browsing
	// and so future Commander runs can reference what was done.
	writeCompletionFile(a.boreDir, task, exec, bossSummary, logf)

	// --- Finish: diff_review ---
	finalExecStatus := db.StatusDiffReview
	if outcome == db.OutcomeFailed {
		finalExecStatus = db.StatusFailed
	}
	logf("phase complete, outcome=%q → status=%q", outcome, finalExecStatus)

	if err := a.db.SetExecutionFinished(ctx, exec.ID, finalExecStatus); err != nil {
		logf("set finished failed: %v", err)
	}
	if err := a.db.UpdateTaskStatus(ctx, task.ID, finalExecStatus); err != nil {
		logf("update task status failed: %v", err)
	}
	a.Emit("tasks_updated", "{}")
	a.Emit("executions_updated", "{}")

	logf("=== DONE: %q (outcome=%s) ===", task.Title, outcome)
	return nil
}

// createExecution creates the DB execution record and the git worktree.
func createExecution(ctx context.Context, a *App, task db.Task, brief agents.ExecutionBrief, logf func(string, ...any)) (*db.Execution, string, error) {
	// Find the crew by name if specified.
	var crewID *int64
	if brief.Crew != "" && brief.Crew != "none" {
		crews, _ := a.db.ListCrews(ctx, a.cluster.ID)
		for _, c := range crews {
			if strings.EqualFold(c.Name, brief.Crew) {
				id := c.ID
				crewID = &id
				logf("matched crew %q (id=%d)", c.Name, id)
				break
			}
		}
		if crewID == nil {
			logf("WARN: commander suggested crew %q but it was not found", brief.Crew)
		}
	}

	// Determine thread name for branch slug.
	threadName := brief.Thread
	if threadName == "" {
		threadName = "general"
	}

	// Derive branch and worktree path.
	execBranch := git.MakeExecBranch(threadName, task.ID, task.Title)
	worktreePath := filepath.Join(a.boreDir, "worktrees", fmt.Sprintf("task-%d", task.ID))

	logf("creating worktree: branch=%q path=%q base=%q", execBranch, worktreePath, brief.BaseBranch)
	if err := a.repo.CreateWorktreeNewBranch(ctx, worktreePath, execBranch, brief.BaseBranch); err != nil {
		return nil, "", fmt.Errorf("taskrunner: create worktree: %w", err)
	}

	exec, err := a.db.CreateExecution(ctx, task.ID, a.cluster.ID, crewID, brief.BaseBranch, execBranch, worktreePath)
	if err != nil {
		return nil, "", fmt.Errorf("taskrunner: create execution record: %w", err)
	}
	return exec, worktreePath, nil
}

// runBossWorkersPhase runs the Boss plan + Workers sequentially, records all
// agent runs and events, and returns the overall outcome string and the boss summary.
// Workers run one at a time to avoid API rate limits.
func runBossWorkersPhase(ctx context.Context, a *App, task db.Task, exec *db.Execution, brief agents.ExecutionBrief, worktreePath string, logf func(string, ...any)) (string, agents.BossSummary, error) {
	// Resolve crew for Boss context.
	var crew *db.Crew
	if exec.CrewID != nil {
		c, err := a.db.GetCrew(ctx, *exec.CrewID)
		if err != nil {
			logf("WARN: could not load crew %d: %v", *exec.CrewID, err)
		} else {
			crew = c
		}
	}

	workerBudget := brief.WorkerBudget
	if workerBudget <= 0 {
		workerBudget = 2
	}

	allCrews, _ := a.db.ListCrews(ctx, a.cluster.ID)

	bossCtx := agents.BossContext{
		Crew:         crew,
		AllCrews:     allCrews,
		Brief:        brief,
		TaskPrompt:   task.Prompt,
		Mode:         task.Mode,
		WorkerBudget: workerBudget,
	}

	// Build a crew lookup map for fast per-worker resolution.
	crewByName := make(map[string]*db.Crew, len(allCrews))
	for i := range allCrews {
		crewByName[strings.ToLower(allCrews[i].Name)] = &allCrews[i]
	}
	bossSys := agents.BuildBossSystemPrompt(bossCtx)
	bossPlanPrompt := agents.BuildBossPlanPrompt(bossCtx)

	// Run Boss plan.
	logf("boss: generating plan (budget=%d)", workerBudget)
	if err := a.db.CreateEvent(ctx, exec.ID, db.LevelInfo, "boss_plan_start", "Boss is generating execution plan"); err != nil {
		logf("WARN: create event: %v", err)
	}
	a.Emit("executions_updated", "{}")

	planResult := a.runner.RunWithSystem(ctx, worktreePath, bossSys, bossPlanPrompt, nil,
		func(line string) { logf("boss stdout: %s", line) },
		func(line string) { logf("boss stderr: %s", line) },
	)
	if planResult.Err != nil {
		return db.OutcomeFailed, agents.BossSummary{}, fmt.Errorf("taskrunner: boss plan run: %w", planResult.Err)
	}
	logf("boss plan raw JSON: %q", planResult.JSONBlock)

	var bossPlan agents.BossPlan
	if planResult.JSONBlock != "" {
		parsed, err := agents.ParseResponse(planResult.JSONBlock)
		if err != nil {
			logf("WARN: failed to parse boss plan: %v", err)
		} else if bp, ok := parsed.(agents.BossPlan); ok {
			bossPlan = bp
			logf("boss plan: %d steps, %d workers needed", len(bp.Steps), len(bp.NeedsWorkers))
		}
	}

	// Record Boss plan agent run.
	if _, err := a.db.CreateAgentRun(ctx, exec.ID, db.AgentTypeBoss, "planner",
		bossPlanPrompt, fmt.Sprintf("%d steps planned", len(bossPlan.Steps)), db.OutcomeSuccess, ""); err != nil {
		logf("WARN: create agent run: %v", err)
	}
	if err := a.db.CreateEvent(ctx, exec.ID, db.LevelInfo, "boss_plan_done",
		fmt.Sprintf("Boss plan ready: %d steps, %d workers", len(bossPlan.Steps), len(bossPlan.NeedsWorkers))); err != nil {
		logf("WARN: create event: %v", err)
	}
	a.Emit("executions_updated", "{}")

	// Cap workers to budget.
	workerNeeds := bossPlan.NeedsWorkers
	if len(workerNeeds) > workerBudget {
		logf("WARN: boss requested %d workers but budget is %d — capping", len(workerNeeds), workerBudget)
		workerNeeds = workerNeeds[:workerBudget]
	}
	if len(workerNeeds) == 0 {
		logf("WARN: boss produced no worker needs — creating a single fallback worker")
		workerNeeds = []agents.WorkerNeed{{
			Role:            "implementer",
			Goal:            task.Prompt,
			SuccessCriteria: brief.SuccessCriteria,
		}}
	}

	// Run workers SEQUENTIALLY to avoid API rate limits.
	logf("running %d workers sequentially", len(workerNeeds))
	if err := a.db.CreateEvent(ctx, exec.ID, db.LevelInfo, "workers_start",
		fmt.Sprintf("Running %d workers sequentially", len(workerNeeds))); err != nil {
		logf("WARN: create event: %v", err)
	}
	a.Emit("executions_updated", "{}")

	var collected []agents.WorkerResult
	anyFailed := false

	for i, need := range workerNeeds {
		logf("worker[%d] %q: starting", i, need.Role)
		if err := a.db.CreateEvent(ctx, exec.ID, db.LevelInfo, "worker_start",
			fmt.Sprintf("Worker %d (%s): starting", i+1, need.Role)); err != nil {
			logf("WARN: create event: %v", err)
		}
		a.Emit("executions_updated", "{}")

		// Resolve the crew for this specific worker: prefer the crew Boss assigned
		// in the plan, fall back to the execution-level crew.
		workerCrew := crew
		if need.Crew != "" && need.Crew != "none" {
			if c, ok := crewByName[strings.ToLower(need.Crew)]; ok {
				workerCrew = c
				logf("worker[%d] %q: using team %q", i, need.Role, workerCrew.Name)
			} else {
				logf("WARN: worker[%d] %q: boss assigned unknown team %q — using execution crew", i, need.Role, need.Crew)
			}
		}

		var workerCrewName, crewObj, crewCons, crewCmds, crewPaths string
		if workerCrew != nil {
			workerCrewName = workerCrew.Name
			crewObj = workerCrew.Objective
			crewCons = workerCrew.Constraints
			crewCmds = workerCrew.AllowedCommands
			crewPaths = workerCrew.OwnershipPaths
		}
		workerSys := agents.BuildWorkerSystemPrompt(agents.WorkerContext{
			Role:                need.Role,
			Goal:                need.Goal,
			FilesOrPaths:        need.FilesOrPaths,
			AllowedCommands:     need.Commands,
			SuccessCriteria:     need.SuccessCriteria,
			CrewName:            workerCrewName,
			CrewObjective:       crewObj,
			CrewConstraints:     crewCons,
			CrewAllowedCommands: crewCmds,
			CrewOwnershipPaths:  crewPaths,
		})

		wr := a.runner.RunWithSystem(ctx, worktreePath, workerSys, need.Goal, nil,
			func(line string) { logf("worker[%d] stdout: %s", i, line) },
			func(line string) { logf("worker[%d] stderr: %s", i, line) },
		)

		var result agents.WorkerResult
		if wr.Err != nil {
			logf("worker[%d] %q run error: %v", i, need.Role, wr.Err)
			result = agents.WorkerResult{Outcome: db.OutcomeFailed, Summary: wr.Err.Error()}
			anyFailed = true
			if err := a.db.CreateEvent(ctx, exec.ID, db.LevelError, "worker_error",
				fmt.Sprintf("Worker %d (%s) error: %v", i+1, need.Role, wr.Err)); err != nil {
				logf("WARN: create event: %v", err)
			}
		} else {
			logf("worker[%d] %q raw JSON: %q", i, need.Role, wr.JSONBlock)
			if wr.JSONBlock != "" {
				parsed, err := agents.ParseResponse(wr.JSONBlock)
				if err != nil {
					logf("WARN: worker[%d] parse error: %v", i, err)
					result = agents.WorkerResult{Outcome: db.OutcomePartial, Summary: wr.Stdout}
				} else if res, ok := parsed.(agents.WorkerResult); ok {
					result = res
				}
			} else {
				result = agents.WorkerResult{Outcome: db.OutcomePartial, Summary: wr.Stdout}
			}
			logf("worker[%d] %q done: outcome=%q", i, need.Role, result.Outcome)
			if result.Outcome == db.OutcomeFailed {
				anyFailed = true
			}
		}

		// Record worker agent run.
		filesStr := strings.Join(result.FilesChanged, ", ")
		if _, err := a.db.CreateAgentRun(ctx, exec.ID, db.AgentTypeWorker, need.Role,
			need.Goal, result.Summary, result.Outcome, filesStr); err != nil {
			logf("WARN: create agent run: %v", err)
		}
		if err := a.db.CreateEvent(ctx, exec.ID, db.LevelInfo, "worker_done",
			fmt.Sprintf("Worker %d (%s): %s — %s", i+1, need.Role, result.Outcome, result.Summary)); err != nil {
			logf("WARN: create event: %v", err)
		}
		a.Emit("executions_updated", "{}")

		collected = append(collected, result)
	}
	logf("all workers done")

	// Run Boss summary.
	logf("boss: generating summary")
	if err := a.db.CreateEvent(ctx, exec.ID, db.LevelInfo, "boss_summary_start", "Boss is summarising worker results"); err != nil {
		logf("WARN: create event: %v", err)
	}
	a.Emit("executions_updated", "{}")

	summaryPrompt := agents.BuildBossSummaryPrompt(collected)
	summaryResult := a.runner.RunWithSystem(ctx, worktreePath, bossSys, summaryPrompt, nil,
		func(line string) { logf("boss stdout: %s", line) },
		func(line string) { logf("boss stderr: %s", line) },
	)
	if summaryResult.Err != nil {
		logf("WARN: boss summary run error: %v", summaryResult.Err)
	}
	logf("boss summary raw JSON: %q", summaryResult.JSONBlock)

	outcome := db.OutcomeSuccess
	if anyFailed {
		outcome = db.OutcomePartial
	}
	var summary agents.BossSummary
	if summaryResult.JSONBlock != "" {
		parsed, err := agents.ParseResponse(summaryResult.JSONBlock)
		if err != nil {
			logf("WARN: failed to parse boss summary: %v", err)
		} else if bs, ok := parsed.(agents.BossSummary); ok {
			summary = bs
			outcome = bs.Outcome
			// Persist lessons.
			for _, l := range bs.Lessons {
				if err := a.db.CreateLesson(ctx, exec.ID, db.AgentTypeBoss, l.LessonType, l.Content); err != nil {
					logf("WARN: create event: %v", err)
				}
			}
			logf("boss summary: outcome=%q, %d changes, %d lessons", bs.Outcome, len(bs.WhatChanged), len(bs.Lessons))
		}
	}

	// Record Boss summary agent run.
	filesStr := strings.Join(summary.FilesTouched, ", ")
	summaryText := strings.Join(summary.WhatChanged, "; ")
	if _, err := a.db.CreateAgentRun(ctx, exec.ID, db.AgentTypeBoss, "summarizer",
		summaryPrompt, summaryText, outcome, filesStr); err != nil {
		logf("WARN: create agent run: %v", err)
	}
	if err := a.db.CreateEvent(ctx, exec.ID, db.LevelInfo, "boss_summary_done",
		fmt.Sprintf("Boss summary: %s", outcome)); err != nil {
		logf("WARN: create event: %v", err)
	}
	a.Emit("executions_updated", "{}")

	return outcome, summary, nil
}

// writeCompletionFile writes a human-readable markdown record of what was done
// to .bore/runs/task-{id}.md. This gives Commander (and humans) a persistent
// file-level view of every completed execution.
func writeCompletionFile(boreDir string, task db.Task, exec *db.Execution, summary agents.BossSummary, logf func(string, ...any)) {
	runsDir := filepath.Join(boreDir, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		logf("WARN: create runs dir: %v", err)
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Task: %s\n\n", task.Title)
	fmt.Fprintf(&b, "**Task ID:** %d  \n", task.ID)
	fmt.Fprintf(&b, "**Execution ID:** %d  \n", exec.ID)
	fmt.Fprintf(&b, "**Branch:** %s  \n", exec.ExecBranch)
	fmt.Fprintf(&b, "**Outcome:** %s  \n\n", summary.Outcome)

	if len(summary.WhatChanged) > 0 {
		b.WriteString("## What Changed\n\n")
		for _, c := range summary.WhatChanged {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteByte('\n')
	}

	if len(summary.FilesTouched) > 0 {
		b.WriteString("## Files Touched\n\n")
		for _, f := range summary.FilesTouched {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		b.WriteByte('\n')
	}

	if len(summary.ValidationResults) > 0 {
		b.WriteString("## Validation\n\n")
		for _, v := range summary.ValidationResults {
			fmt.Fprintf(&b, "- %s\n", v)
		}
		b.WriteByte('\n')
	}

	if len(summary.RisksOrFollowups) > 0 {
		b.WriteString("## Risks / Follow-ups\n\n")
		for _, r := range summary.RisksOrFollowups {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteByte('\n')
	}

	if len(summary.Lessons) > 0 {
		b.WriteString("## Lessons Learned\n\n")
		for _, l := range summary.Lessons {
			fmt.Fprintf(&b, "- [%s] %s\n", l.LessonType, l.Content)
		}
		b.WriteByte('\n')
	}

	filename := filepath.Join(runsDir, fmt.Sprintf("task-%d.md", task.ID))
	if err := os.WriteFile(filename, []byte(b.String()), 0o644); err != nil {
		logf("WARN: write completion file: %v", err)
		return
	}
	logf("completion record written: %s", filename)
}