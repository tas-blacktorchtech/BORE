package db

import "time"

// ---------------------------------------------------------------------------
// Complexity constants
// ---------------------------------------------------------------------------

const (
	ComplexityBasic   = "basic"
	ComplexityMedium  = "medium"
	ComplexityComplex = "complex"
)

// validComplexities is the set of allowed complexity values.
var validComplexities = map[string]bool{
	ComplexityBasic:   true,
	ComplexityMedium:  true,
	ComplexityComplex: true,
}

// ValidComplexity reports whether s is an allowed complexity value.
func ValidComplexity(s string) bool { return validComplexities[s] }

// ---------------------------------------------------------------------------
// Mode constants
// ---------------------------------------------------------------------------

const (
	ModeJustGetItDone   = "just_get_it_done"
	ModeAlertWithIssues = "alert_with_issues"
)

// validModes is the set of allowed mode values.
var validModes = map[string]bool{
	ModeJustGetItDone:   true,
	ModeAlertWithIssues: true,
}

// ValidMode reports whether s is an allowed mode value.
func ValidMode(s string) bool { return validModes[s] }

// ---------------------------------------------------------------------------
// Task / Execution status constants
// ---------------------------------------------------------------------------

const (
	StatusPending     = "pending"
	StatusReview      = "review"
	StatusRunning     = "running"
	StatusDiffReview  = "diff_review"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusInterrupted = "interrupted"
)

// validTaskStatuses is the set of allowed task status values.
var validTaskStatuses = map[string]bool{
	StatusPending:     true,
	StatusReview:      true,
	StatusRunning:     true,
	StatusDiffReview:  true,
	StatusCompleted:   true,
	StatusFailed:      true,
	StatusInterrupted: true,
}

// ValidTaskStatus reports whether s is an allowed task status value.
func ValidTaskStatus(s string) bool { return validTaskStatuses[s] }

// validExecutionStatuses is the set of allowed execution status values.
var validExecutionStatuses = map[string]bool{
	StatusPending:     true,
	StatusReview:      true,
	StatusRunning:     true,
	StatusDiffReview:  true,
	StatusCompleted:   true,
	StatusFailed:      true,
	StatusInterrupted: true,
}

// ValidExecutionStatus reports whether s is an allowed execution status value.
func ValidExecutionStatus(s string) bool { return validExecutionStatuses[s] }

// ---------------------------------------------------------------------------
// Review phase constants
// ---------------------------------------------------------------------------

const (
	PhaseClarification        = "clarification"
	PhaseClarificationAnswers = "clarification_answers"
	PhaseOptions              = "options"
	PhaseSelection            = "selection"
	PhaseBaseBranch           = "base_branch"
)

// ---------------------------------------------------------------------------
// Agent type constants
// ---------------------------------------------------------------------------

const (
	AgentTypeBoss   = "boss"
	AgentTypeWorker = "worker"
)

// ---------------------------------------------------------------------------
// Outcome constants
// ---------------------------------------------------------------------------

const (
	OutcomeSuccess = "success"
	OutcomePartial = "partial"
	OutcomeFailed  = "failed"
)

// ---------------------------------------------------------------------------
// Lesson type constants
// ---------------------------------------------------------------------------

const (
	LessonTypeError   = "error"
	LessonTypePattern = "pattern"
	LessonTypeWarning = "warning"
	LessonTypeNote    = "note"
)

// ---------------------------------------------------------------------------
// Event level constants
// ---------------------------------------------------------------------------

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Cluster represents a git repository workspace managed by bore.
type Cluster struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	RepoPath  string    `json:"repo_path"`
	RemoteURL *string   `json:"remote_url"`
	CreatedAt time.Time `json:"created_at"`
}

// CommanderMemory stores key-value pairs scoped to a cluster for the commander agent.
type CommanderMemory struct {
	ID        int64     `json:"id"`
	ClusterID int64     `json:"cluster_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Crew defines a specialized agent team with constraints and ownership rules.
type Crew struct {
	ID              int64     `json:"id"`
	ClusterID       int64     `json:"cluster_id"`
	Name            string    `json:"name"`
	Objective       string    `json:"objective"`
	Constraints     string    `json:"constraints"`
	AllowedCommands string    `json:"allowed_commands"`
	OwnershipPaths  string    `json:"ownership_paths"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Thread groups related tasks under a named context within a cluster.
type Thread struct {
	ID          int64     `json:"id"`
	ClusterID   int64     `json:"cluster_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Task represents a unit of work assigned within a thread.
type Task struct {
	ID         int64     `json:"id"`
	ClusterID  int64     `json:"cluster_id"`
	ThreadID   int64     `json:"thread_id"`
	Title      string    `json:"title"`
	Prompt     string    `json:"prompt"`
	Complexity string    `json:"complexity"`
	Mode       string    `json:"mode"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TaskReview captures review phase data for a task.
type TaskReview struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	Phase     string    `json:"phase"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Execution tracks a single run of a task, potentially by a crew.
type Execution struct {
	ID           int64      `json:"id"`
	TaskID       int64      `json:"task_id"`
	ClusterID    int64      `json:"cluster_id"`
	CrewID       *int64     `json:"crew_id"`
	BaseBranch   string     `json:"base_branch"`
	ExecBranch   string     `json:"exec_branch"`
	WorktreePath string     `json:"worktree_path"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ExecutionEvent is a timestamped log entry for an execution.
type ExecutionEvent struct {
	ID          int64     `json:"id"`
	ExecutionID int64     `json:"execution_id"`
	Ts          time.Time `json:"ts"`
	Level       string    `json:"level"`
	EventType   string    `json:"event_type"`
	Message     string    `json:"message"`
}

// AgentRun records one agent invocation within an execution.
type AgentRun struct {
	ID           int64     `json:"id"`
	ExecutionID  int64     `json:"execution_id"`
	AgentType    string    `json:"agent_type"`
	Role         string    `json:"role"`
	Prompt       string    `json:"prompt"`
	Summary      string    `json:"summary"`
	Outcome      string    `json:"outcome"`
	FilesChanged string    `json:"files_changed"`
	CreatedAt    time.Time `json:"created_at"`
}

// AgentLesson captures a lesson learned during an agent run.
type AgentLesson struct {
	ID          int64     `json:"id"`
	ExecutionID int64     `json:"execution_id"`
	AgentType   string    `json:"agent_type"`
	LessonType  string    `json:"lesson_type"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}
