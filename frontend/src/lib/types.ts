// Status types
export type TaskStatus = 'pending' | 'review' | 'running' | 'diff_review' | 'completed' | 'failed' | 'interrupted'
export type Complexity = 'basic' | 'medium' | 'complex'
export type Mode = 'just_get_it_done' | 'alert_with_issues'
export type AgentType = 'boss' | 'worker'
export type Outcome = 'success' | 'partial' | 'failed'
export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

// Models
export interface Cluster {
  id: number
  name: string
  repo_path: string
  remote_url: string | null
  created_at: string
}

export interface Task {
  id: number
  cluster_id: number
  thread_id: number
  title: string
  prompt: string
  complexity: Complexity
  mode: Mode
  status: TaskStatus
  created_at: string
  updated_at: string
}

export interface Thread {
  id: number
  cluster_id: number
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface Crew {
  id: number
  cluster_id: number
  name: string
  objective: string
  constraints: string
  allowed_commands: string
  ownership_paths: string
  created_at: string
  updated_at: string
}

export interface Execution {
  id: number
  task_id: number
  cluster_id: number
  crew_id: number | null
  base_branch: string
  exec_branch: string
  worktree_path: string
  status: TaskStatus
  started_at: string | null
  finished_at: string | null
  created_at: string
  updated_at: string
}

export interface ExecutionEvent {
  id: number
  execution_id: number
  ts: string
  level: LogLevel
  event_type: string
  message: string
}

export interface AgentRun {
  id: number
  execution_id: number
  agent_type: AgentType
  role: string
  prompt: string
  summary: string
  outcome: Outcome
  files_changed: string
  created_at: string
}

// Task Reviews (Commander review flow)
export type ReviewPhase = 'clarification' | 'clarification_answers' | 'options' | 'selection' | 'base_branch'

export interface TaskReview {
  id: number
  task_id: number
  phase: ReviewPhase
  content: string
  created_at: string
}

export interface ClarificationQuestion {
  id: string
  question: string
  why: string
}

export interface ClarificationsResponse {
  type: string
  questions: ClarificationQuestion[]
}

export interface ExecutionOption {
  id: string
  title: string
  summary: string
  approach_steps: string[]
  crew_suggestion: string
  worker_budget_suggestion: number
  risks: string[]
  validation: string[]
}

export interface OptionsResponse {
  type: string
  options: ExecutionOption[]
}

// API Responses
export interface StatusResponse {
  has_cluster: boolean
  cluster: Cluster | null
  port: number
}

export interface ServerInfoResponse {
  port: number
  bind: string
  local_ips: string[] | null
  network_mode: boolean
}

export interface DiffResponse {
  status: string
  diff: string
}

export interface ChatMessage {
  role: 'user' | 'commander'
  content: string
}

export interface KnownCluster {
  path: string
  name: string
}

// Form inputs
export interface CreateTaskInput {
  title: string
  prompt: string
  complexity: Complexity
  mode: Mode
  thread_id: number | null
}

export interface CreateCrewInput {
  name: string
  objective: string
  constraints: string
  allowed_commands: string
  ownership_paths: string
}

export interface CreateThreadInput {
  name: string
  description: string
}
