package agents

// ClarificationQuestion is a single clarifying question from Commander.
type ClarificationQuestion struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Why      string `json:"why"`
}

// ClarificationsResponse is the Commander's clarification output.
type ClarificationsResponse struct {
	Type      string                  `json:"type"`
	Questions []ClarificationQuestion `json:"questions"`
}

// ExecutionOption is one of the Commander's proposed execution approaches.
type ExecutionOption struct {
	ID                     string   `json:"id"`
	Title                  string   `json:"title"`
	Summary                string   `json:"summary"`
	ApproachSteps          []string `json:"approach_steps"`
	CrewSuggestion         string   `json:"crew_suggestion"`
	WorkerBudgetSuggestion int      `json:"worker_budget_suggestion"`
	Risks                  []string `json:"risks"`
	Validation             []string `json:"validation"`
}

// OptionsResponse is the Commander's options output.
type OptionsResponse struct {
	Type    string            `json:"type"`
	Options []ExecutionOption `json:"options"`
}

// ExecutionBrief is the Commander's final execution plan.
type ExecutionBrief struct {
	Type                  string   `json:"type"`
	SelectedOptionID      string   `json:"selected_option_id"`
	BaseBranch            string   `json:"base_branch"`
	Thread                string   `json:"thread"`
	TaskTitle             string   `json:"task_title"`
	Scope                 []string `json:"scope"`
	NotInScope            []string `json:"not_in_scope"`
	SuccessCriteria       []string `json:"success_criteria"`
	Crew                  string   `json:"crew"`
	WorkerBudget          int      `json:"worker_budget"`
	KeyRisks              []string `json:"key_risks"`
	RecommendedValidation []string `json:"recommended_validation"`
}

// BossPlanStep is a step in the Boss's execution plan.
type BossPlanStep struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	WorkerRole string `json:"worker_role"`
}

// WorkerNeed describes a worker the Boss wants to spawn.
type WorkerNeed struct {
	Role            string   `json:"role"`
	Goal            string   `json:"goal"`
	Crew            string   `json:"crew"`             // team name to assign (may be empty)
	FilesOrPaths    []string `json:"files_or_paths"`
	Commands        []string `json:"commands"`
	SuccessCriteria []string `json:"success_criteria"`
}

// BossPlan is the Boss's initial execution plan.
type BossPlan struct {
	Type           string         `json:"type"`
	Steps          []BossPlanStep `json:"steps"`
	Validation     []string       `json:"validation"`
	EstimatedFiles []string       `json:"estimated_files"`
	NeedsWorkers   []WorkerNeed   `json:"needs_workers"`
}

// SpawnWorkersRequest is the Boss asking to spawn workers.
type SpawnWorkersRequest struct {
	Type    string       `json:"type"`
	Workers []WorkerNeed `json:"workers"`
}

// BossLesson is a lesson extracted by the Boss.
type BossLesson struct {
	LessonType string `json:"lesson_type"`
	Content    string `json:"content"`
}

// BossSummary is the Boss's final summary after execution.
type BossSummary struct {
	Type              string       `json:"type"`
	Outcome           string       `json:"outcome"`
	WhatChanged       []string     `json:"what_changed"`
	FilesTouched      []string     `json:"files_touched"`
	CommandsRun       []string     `json:"commands_run"`
	ValidationResults []string     `json:"validation_results"`
	RisksOrFollowups  []string     `json:"risks_or_followups"`
	Lessons           []BossLesson `json:"lessons"`
}

// WorkerResult is the Worker's output after completing work.
type WorkerResult struct {
	Type              string   `json:"type"`
	Outcome           string   `json:"outcome"`
	Summary           string   `json:"summary"`
	FilesChanged      []string `json:"files_changed"`
	CommandsRun       []string `json:"commands_run"`
	ValidationResults []string `json:"validation_results"`
	Notes             []string `json:"notes"`
	Blockers          []string `json:"blockers"`
}
