PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS clusters (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  repo_path TEXT NOT NULL UNIQUE,
  remote_url TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS commander_memory (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cluster_id INTEGER NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(cluster_id, key),
  FOREIGN KEY(cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS crews (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cluster_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  objective TEXT NOT NULL,
  constraints TEXT NOT NULL DEFAULT '',
  allowed_commands TEXT NOT NULL DEFAULT '',
  ownership_paths TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(cluster_id, name),
  FOREIGN KEY(cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS threads (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cluster_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(cluster_id, name),
  FOREIGN KEY(cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cluster_id INTEGER NOT NULL,
  thread_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  prompt TEXT NOT NULL,
  complexity TEXT NOT NULL CHECK (complexity IN ('basic','medium','complex')),
  mode TEXT NOT NULL CHECK (mode IN ('just_get_it_done','alert_with_issues')),
  status TEXT NOT NULL CHECK (status IN ('pending','review','running','diff_review','completed','failed','interrupted')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
  FOREIGN KEY(thread_id) REFERENCES threads(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS task_reviews (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id INTEGER NOT NULL,
  phase TEXT NOT NULL CHECK (phase IN ('clarification','options','selection','base_branch')),
  content TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS executions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id INTEGER NOT NULL,
  cluster_id INTEGER NOT NULL,
  crew_id INTEGER,
  base_branch TEXT NOT NULL,
  exec_branch TEXT NOT NULL,
  worktree_path TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','review','running','diff_review','completed','failed','interrupted')),
  started_at TEXT,
  finished_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  FOREIGN KEY(cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
  FOREIGN KEY(crew_id) REFERENCES crews(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS execution_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  execution_id INTEGER NOT NULL,
  ts TEXT NOT NULL,
  level TEXT NOT NULL CHECK (level IN ('debug','info','warn','error')),
  event_type TEXT NOT NULL,
  message TEXT NOT NULL,
  FOREIGN KEY(execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS agent_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  execution_id INTEGER NOT NULL,
  agent_type TEXT NOT NULL CHECK (agent_type IN ('boss','worker')),
  role TEXT NOT NULL,
  prompt TEXT NOT NULL,
  summary TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('success','partial','failed')),
  files_changed TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY(execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS agent_lessons (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  execution_id INTEGER NOT NULL,
  agent_type TEXT NOT NULL CHECK (agent_type IN ('boss','worker')),
  lesson_type TEXT NOT NULL CHECK (lesson_type IN ('error','pattern','warning','note')),
  content TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tasks_cluster_thread ON tasks(cluster_id, thread_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_exec_task ON executions(task_id);
CREATE INDEX IF NOT EXISTS idx_exec_cluster ON executions(cluster_id);
CREATE INDEX IF NOT EXISTS idx_exec_status ON executions(cluster_id, status);
CREATE INDEX IF NOT EXISTS idx_agent_runs_exec ON agent_runs(execution_id);
CREATE INDEX IF NOT EXISTS idx_events_exec_ts ON execution_events(execution_id, ts);
