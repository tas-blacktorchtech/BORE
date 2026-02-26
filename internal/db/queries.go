package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// scanner is the common interface satisfied by both *sql.Row and *sql.Rows,
// allowing a single scan function per entity.
type scanner interface {
	Scan(dest ...any) error
}

// ErrNotFound is returned when a delete or update targets a non-existent row.
var ErrNotFound = errors.New("record not found")

// now returns the current time formatted as RFC3339 for storage.
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// parseTime parses an RFC3339 string into time.Time.
func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// parseNullableTime parses a sql.NullString into *time.Time.
func parseNullableTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// nullString converts a Go string to sql.NullString (empty string -> NULL).
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullableToPtr converts sql.NullString to *string.
func nullableToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// nullableInt64ToPtr converts sql.NullInt64 to *int64.
func nullableInt64ToPtr(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

// ptrToNullInt64 converts *int64 to sql.NullInt64.
func ptrToNullInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// ptrFromString returns nil for empty strings, otherwise a pointer to s.
func ptrFromString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------------------------------------------------------------------------
// Clusters
// ---------------------------------------------------------------------------

// CreateCluster inserts a new cluster and returns it.
func (d *DB) CreateCluster(ctx context.Context, name, repoPath, remoteURL string) (*Cluster, error) {
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO clusters (name, repo_path, remote_url, created_at)
		 VALUES (?, ?, ?, ?)`,
		name, repoPath, nullString(remoteURL), ts,
	)
	if err != nil {
		return nil, fmt.Errorf("create cluster: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create cluster: last insert id: %w", err)
	}
	createdAt, err := parseTime(ts)
	if err != nil {
		return nil, fmt.Errorf("create cluster: parse time: %w", err)
	}
	return &Cluster{
		ID:        id,
		Name:      name,
		RepoPath:  repoPath,
		RemoteURL: ptrFromString(remoteURL),
		CreatedAt: createdAt,
	}, nil
}

// GetCluster returns a single cluster by ID.
func (d *DB) GetCluster(ctx context.Context, id int64) (*Cluster, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, name, repo_path, remote_url, created_at FROM clusters WHERE id = ?`, id,
	)
	return scanCluster(row)
}

// ListClusters returns all clusters ordered by name.
func (d *DB) ListClusters(ctx context.Context) ([]Cluster, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, name, repo_path, remote_url, created_at FROM clusters ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	defer rows.Close()

	var out []Cluster
	for rows.Next() {
		c, err := scanCluster(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// GetClusterByPath returns the cluster with the given repo_path, or ErrNotFound.
func (d *DB) GetClusterByPath(ctx context.Context, repoPath string) (*Cluster, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, name, repo_path, remote_url, created_at FROM clusters WHERE repo_path = ?`, repoPath,
	)
	c, err := scanCluster(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get cluster by path (%s): %w", repoPath, ErrNotFound)
		}
		return nil, fmt.Errorf("get cluster by path: %w", err)
	}
	return c, nil
}

// DeleteCluster removes a cluster by ID. Cascading deletes handle children.
func (d *DB) DeleteCluster(ctx context.Context, id int64) error {
	res, err := d.conn.ExecContext(ctx, `DELETE FROM clusters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete cluster: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete cluster (id=%d): %w", id, ErrNotFound)
	}
	return nil
}

func scanCluster(s scanner) (*Cluster, error) {
	var c Cluster
	var remoteURL sql.NullString
	var createdAt string
	if err := s.Scan(&c.ID, &c.Name, &c.RepoPath, &remoteURL, &createdAt); err != nil {
		return nil, fmt.Errorf("scan cluster: %w", err)
	}
	c.RemoteURL = nullableToPtr(remoteURL)
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	c.CreatedAt = t
	return &c, nil
}

// ---------------------------------------------------------------------------
// Commander Memory
// ---------------------------------------------------------------------------

// SetMemory upserts a key-value pair for the given cluster.
func (d *DB) SetMemory(ctx context.Context, clusterID int64, key, value string) error {
	ts := now()
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO commander_memory (cluster_id, key, value, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(cluster_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		clusterID, key, value, ts,
	)
	if err != nil {
		return fmt.Errorf("set memory: %w", err)
	}
	return nil
}

// GetMemory returns the value for a key in a cluster's commander memory.
func (d *DB) GetMemory(ctx context.Context, clusterID int64, key string) (string, error) {
	var value string
	err := d.conn.QueryRowContext(ctx,
		`SELECT value FROM commander_memory WHERE cluster_id = ? AND key = ?`,
		clusterID, key,
	).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get memory: %w", err)
	}
	return value, nil
}

// GetAllMemory returns all memory entries for a cluster.
func (d *DB) GetAllMemory(ctx context.Context, clusterID int64) ([]CommanderMemory, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, cluster_id, key, value, updated_at
		 FROM commander_memory WHERE cluster_id = ? ORDER BY key`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list memory: %w", err)
	}
	defer rows.Close()

	var out []CommanderMemory
	for rows.Next() {
		var m CommanderMemory
		var updatedAt string
		if err := rows.Scan(&m.ID, &m.ClusterID, &m.Key, &m.Value, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMemory removes a single memory entry.
func (d *DB) DeleteMemory(ctx context.Context, clusterID int64, key string) error {
	res, err := d.conn.ExecContext(ctx,
		`DELETE FROM commander_memory WHERE cluster_id = ? AND key = ?`,
		clusterID, key,
	)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete memory: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete memory (cluster_id=%d, key=%q): %w", clusterID, key, ErrNotFound)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Crews
// ---------------------------------------------------------------------------

// CreateCrew inserts a new crew and returns it.
func (d *DB) CreateCrew(ctx context.Context, clusterID int64, name, objective, constraints, allowedCommands, ownershipPaths string) (*Crew, error) {
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO crews (cluster_id, name, objective, constraints, allowed_commands, ownership_paths, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		clusterID, name, objective, constraints, allowedCommands, ownershipPaths, ts, ts,
	)
	if err != nil {
		return nil, fmt.Errorf("create crew: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create crew: last insert id: %w", err)
	}
	createdAt, err := parseTime(ts)
	if err != nil {
		return nil, fmt.Errorf("create crew: parse time: %w", err)
	}
	return &Crew{
		ID:              id,
		ClusterID:       clusterID,
		Name:            name,
		Objective:       objective,
		Constraints:     constraints,
		AllowedCommands: allowedCommands,
		OwnershipPaths:  ownershipPaths,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}, nil
}

// GetCrew returns a crew by ID.
func (d *DB) GetCrew(ctx context.Context, id int64) (*Crew, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, cluster_id, name, objective, constraints, allowed_commands, ownership_paths, created_at, updated_at
		 FROM crews WHERE id = ?`, id,
	)
	return scanCrew(row)
}

// ListCrews returns all crews for a cluster.
func (d *DB) ListCrews(ctx context.Context, clusterID int64) ([]Crew, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, cluster_id, name, objective, constraints, allowed_commands, ownership_paths, created_at, updated_at
		 FROM crews WHERE cluster_id = ? ORDER BY name`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list crews: %w", err)
	}
	defer rows.Close()

	var out []Crew
	for rows.Next() {
		c, err := scanCrew(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// UpdateCrew updates all mutable fields of a crew.
func (d *DB) UpdateCrew(ctx context.Context, crew *Crew) error {
	ts := now()
	_, err := d.conn.ExecContext(ctx,
		`UPDATE crews SET name = ?, objective = ?, constraints = ?, allowed_commands = ?, ownership_paths = ?, updated_at = ?
		 WHERE id = ?`,
		crew.Name, crew.Objective, crew.Constraints, crew.AllowedCommands, crew.OwnershipPaths, ts, crew.ID,
	)
	if err != nil {
		return fmt.Errorf("update crew: %w", err)
	}
	updatedAt, err := parseTime(ts)
	if err != nil {
		return fmt.Errorf("update crew: parse time: %w", err)
	}
	crew.UpdatedAt = updatedAt
	return nil
}

// DeleteCrew removes a crew by ID.
func (d *DB) DeleteCrew(ctx context.Context, id int64) error {
	res, err := d.conn.ExecContext(ctx, `DELETE FROM crews WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete crew: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete crew: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete crew (id=%d): %w", id, ErrNotFound)
	}
	return nil
}

func scanCrew(s scanner) (*Crew, error) {
	var c Crew
	var createdAt, updatedAt string
	if err := s.Scan(&c.ID, &c.ClusterID, &c.Name, &c.Objective, &c.Constraints,
		&c.AllowedCommands, &c.OwnershipPaths, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan crew: %w", err)
	}
	var err error
	c.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	c.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ---------------------------------------------------------------------------
// Threads
// ---------------------------------------------------------------------------

// CreateThread inserts a new thread and returns it.
func (d *DB) CreateThread(ctx context.Context, clusterID int64, name, description string) (*Thread, error) {
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO threads (cluster_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		clusterID, name, description, ts, ts,
	)
	if err != nil {
		return nil, fmt.Errorf("create thread: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create thread: last insert id: %w", err)
	}
	createdAt, err := parseTime(ts)
	if err != nil {
		return nil, fmt.Errorf("create thread: parse time: %w", err)
	}
	return &Thread{
		ID:          id,
		ClusterID:   clusterID,
		Name:        name,
		Description: description,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}, nil
}

// GetThread returns a thread by ID.
func (d *DB) GetThread(ctx context.Context, id int64) (*Thread, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, cluster_id, name, description, created_at, updated_at
		 FROM threads WHERE id = ?`, id,
	)
	return scanThread(row)
}

// ListThreads returns all threads for a cluster.
func (d *DB) ListThreads(ctx context.Context, clusterID int64) ([]Thread, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, cluster_id, name, description, created_at, updated_at
		 FROM threads WHERE cluster_id = ? ORDER BY name`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list threads: %w", err)
	}
	defer rows.Close()

	var out []Thread
	for rows.Next() {
		t, err := scanThread(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// UpdateThread updates a thread's mutable fields.
func (d *DB) UpdateThread(ctx context.Context, thread *Thread) error {
	ts := now()
	_, err := d.conn.ExecContext(ctx,
		`UPDATE threads SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		thread.Name, thread.Description, ts, thread.ID,
	)
	if err != nil {
		return fmt.Errorf("update thread: %w", err)
	}
	updatedAt, err := parseTime(ts)
	if err != nil {
		return fmt.Errorf("update thread: parse time: %w", err)
	}
	thread.UpdatedAt = updatedAt
	return nil
}

func scanThread(s scanner) (*Thread, error) {
	var t Thread
	var createdAt, updatedAt string
	if err := s.Scan(&t.ID, &t.ClusterID, &t.Name, &t.Description, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan thread: %w", err)
	}
	var err error
	t.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	t.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ---------------------------------------------------------------------------
// Tasks
// ---------------------------------------------------------------------------

// CreateTask inserts a new task with initial status "pending" and returns it.
func (d *DB) CreateTask(ctx context.Context, clusterID, threadID int64, title, prompt, complexity, mode string) (*Task, error) {
	if !ValidComplexity(complexity) {
		return nil, fmt.Errorf("create task: invalid complexity %q", complexity)
	}
	if !ValidMode(mode) {
		return nil, fmt.Errorf("create task: invalid mode %q", mode)
	}
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO tasks (cluster_id, thread_id, title, prompt, complexity, mode, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?)`,
		clusterID, threadID, title, prompt, complexity, mode, ts, ts,
	)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create task: last insert id: %w", err)
	}
	createdAt, err := parseTime(ts)
	if err != nil {
		return nil, fmt.Errorf("create task: parse time: %w", err)
	}
	return &Task{
		ID:         id,
		ClusterID:  clusterID,
		ThreadID:   threadID,
		Title:      title,
		Prompt:     prompt,
		Complexity: complexity,
		Mode:       mode,
		Status:     StatusPending,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}, nil
}

// GetTask returns a task by ID, or nil if not found.
func (d *DB) GetTask(ctx context.Context, id int64) (*Task, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, cluster_id, thread_id, title, prompt, complexity, mode, status, created_at, updated_at
		 FROM tasks WHERE id = ?`, id,
	)
	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

// ListTasks returns all tasks for a cluster.
func (d *DB) ListTasks(ctx context.Context, clusterID int64) ([]Task, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, cluster_id, thread_id, title, prompt, complexity, mode, status, created_at, updated_at
		 FROM tasks WHERE cluster_id = ? ORDER BY created_at DESC`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

// ListTasksByThread returns all tasks in a given thread.
func (d *DB) ListTasksByThread(ctx context.Context, threadID int64) ([]Task, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, cluster_id, thread_id, title, prompt, complexity, mode, status, created_at, updated_at
		 FROM tasks WHERE thread_id = ? ORDER BY created_at DESC`,
		threadID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks by thread: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

// ListTasksByStatus returns tasks for a cluster filtered by status.
func (d *DB) ListTasksByStatus(ctx context.Context, clusterID int64, status string) ([]Task, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, cluster_id, thread_id, title, prompt, complexity, mode, status, created_at, updated_at
		 FROM tasks WHERE cluster_id = ? AND status = ? ORDER BY created_at DESC`,
		clusterID, status,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks by status: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

// TaskHistoryEntry holds a task title/prompt/status paired with its boss summary outcome.
// Used to give the Commander a complete view of what has been done.
type TaskHistoryEntry struct {
	TaskID       int64
	Title        string
	Prompt       string
	Status       string
	Outcome      string // boss summarizer outcome: success/partial/failed or empty
	WhatChanged  string // boss summarizer summary text or empty
	FilesChanged string // comma-separated files touched
}

// ListTaskHistories returns up to 20 most recent tasks for a cluster, each joined
// with the boss summarizer outcome from the task's most recent execution (if any).
func (d *DB) ListTaskHistories(ctx context.Context, clusterID int64) ([]TaskHistoryEntry, error) {
	rows, err := d.conn.QueryContext(ctx, `
		SELECT
			t.id,
			t.title,
			t.prompt,
			t.status,
			COALESCE(ar.outcome, '')       AS outcome,
			COALESCE(ar.summary, '')       AS what_changed,
			COALESCE(ar.files_changed, '') AS files_changed
		FROM tasks t
		LEFT JOIN executions e
			ON e.task_id = t.id
			AND e.id = (SELECT MAX(e2.id) FROM executions e2 WHERE e2.task_id = t.id)
		LEFT JOIN agent_runs ar
			ON ar.execution_id = e.id AND ar.role = 'summarizer'
		WHERE t.cluster_id = ?
		ORDER BY t.created_at DESC
		LIMIT 20
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list task histories: %w", err)
	}
	defer rows.Close()

	var out []TaskHistoryEntry
	for rows.Next() {
		var e TaskHistoryEntry
		if err := rows.Scan(&e.TaskID, &e.Title, &e.Prompt, &e.Status,
			&e.Outcome, &e.WhatChanged, &e.FilesChanged); err != nil {
			return nil, fmt.Errorf("scan task history: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateTaskStatus changes a task's status.
func (d *DB) UpdateTaskStatus(ctx context.Context, id int64, status string) error {
	if !ValidTaskStatus(status) {
		return fmt.Errorf("update task status: invalid status %q", status)
	}
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`,
		status, ts, id,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task status: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update task status (id=%d): %w", id, ErrNotFound)
	}
	return nil
}

func scanTask(s scanner) (*Task, error) {
	var t Task
	var createdAt, updatedAt string
	if err := s.Scan(&t.ID, &t.ClusterID, &t.ThreadID, &t.Title, &t.Prompt,
		&t.Complexity, &t.Mode, &t.Status, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}
	var err error
	t.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	t.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func collectTasks(rows *sql.Rows) ([]Task, error) {
	var out []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Task Reviews
// ---------------------------------------------------------------------------

// CreateTaskReview inserts a review phase entry for a task.
func (d *DB) CreateTaskReview(ctx context.Context, taskID int64, phase, content string) (*TaskReview, error) {
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO task_reviews (task_id, phase, content, created_at)
		 VALUES (?, ?, ?, ?)`,
		taskID, phase, content, ts,
	)
	if err != nil {
		return nil, fmt.Errorf("create task review: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create task review: last insert id: %w", err)
	}
	createdAt, err := parseTime(ts)
	if err != nil {
		return nil, fmt.Errorf("create task review: parse time: %w", err)
	}
	return &TaskReview{
		ID:        id,
		TaskID:    taskID,
		Phase:     phase,
		Content:   content,
		CreatedAt: createdAt,
	}, nil
}

// GetTaskReviews returns all reviews for a task ordered by creation time.
func (d *DB) GetTaskReviews(ctx context.Context, taskID int64) ([]TaskReview, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, task_id, phase, content, created_at
		 FROM task_reviews WHERE task_id = ? ORDER BY created_at`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task reviews: %w", err)
	}
	defer rows.Close()

	var out []TaskReview
	for rows.Next() {
		var r TaskReview
		var createdAt string
		if err := rows.Scan(&r.ID, &r.TaskID, &r.Phase, &r.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan task review: %w", err)
		}
		r.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Executions
// ---------------------------------------------------------------------------

// CreateExecution inserts a new execution with initial status "pending".
func (d *DB) CreateExecution(ctx context.Context, taskID, clusterID int64, crewID *int64, baseBranch, execBranch, worktreePath string) (*Execution, error) {
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO executions (task_id, cluster_id, crew_id, base_branch, exec_branch, worktree_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?)`,
		taskID, clusterID, ptrToNullInt64(crewID), baseBranch, execBranch, worktreePath, ts, ts,
	)
	if err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create execution: last insert id: %w", err)
	}
	createdAt, err := parseTime(ts)
	if err != nil {
		return nil, fmt.Errorf("create execution: parse time: %w", err)
	}
	return &Execution{
		ID:           id,
		TaskID:       taskID,
		ClusterID:    clusterID,
		CrewID:       crewID,
		BaseBranch:   baseBranch,
		ExecBranch:   execBranch,
		WorktreePath: worktreePath,
		Status:       StatusPending,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}, nil
}

// GetExecution returns an execution by ID, or nil if not found.
func (d *DB) GetExecution(ctx context.Context, id int64) (*Execution, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, task_id, cluster_id, crew_id, base_branch, exec_branch, worktree_path,
		        status, started_at, finished_at, created_at, updated_at
		 FROM executions WHERE id = ?`, id,
	)
	e, err := scanExecution(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get execution: %w", err)
	}
	return e, nil
}

// ListExecutions returns all executions for a cluster.
func (d *DB) ListExecutions(ctx context.Context, clusterID int64) ([]Execution, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, task_id, cluster_id, crew_id, base_branch, exec_branch, worktree_path,
		        status, started_at, finished_at, created_at, updated_at
		 FROM executions WHERE cluster_id = ? ORDER BY created_at DESC`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()
	return collectExecutions(rows)
}

// ListExecutionsByStatus returns executions for a cluster filtered by status.
func (d *DB) ListExecutionsByStatus(ctx context.Context, clusterID int64, status string) ([]Execution, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, task_id, cluster_id, crew_id, base_branch, exec_branch, worktree_path,
		        status, started_at, finished_at, created_at, updated_at
		 FROM executions WHERE cluster_id = ? AND status = ? ORDER BY created_at DESC`,
		clusterID, status,
	)
	if err != nil {
		return nil, fmt.Errorf("list executions by status: %w", err)
	}
	defer rows.Close()
	return collectExecutions(rows)
}

// UpdateExecutionStatus changes an execution's status.
func (d *DB) UpdateExecutionStatus(ctx context.Context, id int64, status string) error {
	if !ValidExecutionStatus(status) {
		return fmt.Errorf("update execution status: invalid status %q", status)
	}
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`UPDATE executions SET status = ?, updated_at = ? WHERE id = ?`,
		status, ts, id,
	)
	if err != nil {
		return fmt.Errorf("update execution status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update execution status: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update execution status (id=%d): %w", id, ErrNotFound)
	}
	return nil
}

// SetExecutionStarted records the start time and sets status to "running".
func (d *DB) SetExecutionStarted(ctx context.Context, id int64) error {
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`UPDATE executions SET status = 'running', started_at = ?, updated_at = ? WHERE id = ?`,
		ts, ts, id,
	)
	if err != nil {
		return fmt.Errorf("set execution started: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set execution started: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("set execution started (id=%d): %w", id, ErrNotFound)
	}
	return nil
}

// SetExecutionFinished records the finish time and sets the final status.
func (d *DB) SetExecutionFinished(ctx context.Context, id int64, status string) error {
	if !ValidExecutionStatus(status) {
		return fmt.Errorf("set execution finished: invalid status %q", status)
	}
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`UPDATE executions SET status = ?, finished_at = ?, updated_at = ? WHERE id = ?`,
		status, ts, ts, id,
	)
	if err != nil {
		return fmt.Errorf("set execution finished: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set execution finished: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("set execution finished (id=%d): %w", id, ErrNotFound)
	}
	return nil
}

func scanExecution(s scanner) (*Execution, error) {
	var e Execution
	var crewID sql.NullInt64
	var startedAt, finishedAt sql.NullString
	var createdAt, updatedAt string
	if err := s.Scan(&e.ID, &e.TaskID, &e.ClusterID, &crewID,
		&e.BaseBranch, &e.ExecBranch, &e.WorktreePath,
		&e.Status, &startedAt, &finishedAt, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan execution: %w", err)
	}
	e.CrewID = nullableInt64ToPtr(crewID)
	var err error
	e.StartedAt, err = parseNullableTime(startedAt)
	if err != nil {
		return nil, err
	}
	e.FinishedAt, err = parseNullableTime(finishedAt)
	if err != nil {
		return nil, err
	}
	e.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	e.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func collectExecutions(rows *sql.Rows) ([]Execution, error) {
	var out []Execution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Execution Events
// ---------------------------------------------------------------------------

// CreateEvent inserts a timestamped event for an execution.
func (d *DB) CreateEvent(ctx context.Context, executionID int64, level, eventType, message string) error {
	ts := now()
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO execution_events (execution_id, ts, level, event_type, message)
		 VALUES (?, ?, ?, ?, ?)`,
		executionID, ts, level, eventType, message,
	)
	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}

// ListEvents returns all events for an execution ordered by timestamp.
func (d *DB) ListEvents(ctx context.Context, executionID int64) ([]ExecutionEvent, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, execution_id, ts, level, event_type, message
		 FROM execution_events WHERE execution_id = ? ORDER BY ts`,
		executionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var out []ExecutionEvent
	for rows.Next() {
		var ev ExecutionEvent
		var ts string
		if err := rows.Scan(&ev.ID, &ev.ExecutionID, &ts, &ev.Level, &ev.EventType, &ev.Message); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.Ts, err = parseTime(ts)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Agent Runs
// ---------------------------------------------------------------------------

// CreateAgentRun inserts an agent run record.
func (d *DB) CreateAgentRun(ctx context.Context, executionID int64, agentType, role, prompt, summary, outcome, filesChanged string) (*AgentRun, error) {
	ts := now()
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO agent_runs (execution_id, agent_type, role, prompt, summary, outcome, files_changed, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		executionID, agentType, role, prompt, summary, outcome, filesChanged, ts,
	)
	if err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create agent run: last insert id: %w", err)
	}
	createdAt, err := parseTime(ts)
	if err != nil {
		return nil, fmt.Errorf("create agent run: parse time: %w", err)
	}
	return &AgentRun{
		ID:           id,
		ExecutionID:  executionID,
		AgentType:    agentType,
		Role:         role,
		Prompt:       prompt,
		Summary:      summary,
		Outcome:      outcome,
		FilesChanged: filesChanged,
		CreatedAt:    createdAt,
	}, nil
}

// GetAgentRuns returns all agent runs for an execution.
func (d *DB) GetAgentRuns(ctx context.Context, executionID int64) ([]AgentRun, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, execution_id, agent_type, role, prompt, summary, outcome, files_changed, created_at
		 FROM agent_runs WHERE execution_id = ? ORDER BY created_at`,
		executionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent runs: %w", err)
	}
	defer rows.Close()
	return collectAgentRuns(rows)
}

// GetAgentRunsByType returns agent runs filtered by type (boss or worker).
func (d *DB) GetAgentRunsByType(ctx context.Context, executionID int64, agentType string) ([]AgentRun, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, execution_id, agent_type, role, prompt, summary, outcome, files_changed, created_at
		 FROM agent_runs WHERE execution_id = ? AND agent_type = ? ORDER BY created_at`,
		executionID, agentType,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent runs by type: %w", err)
	}
	defer rows.Close()
	return collectAgentRuns(rows)
}

func scanAgentRun(s scanner) (*AgentRun, error) {
	var r AgentRun
	var createdAt string
	if err := s.Scan(&r.ID, &r.ExecutionID, &r.AgentType, &r.Role,
		&r.Prompt, &r.Summary, &r.Outcome, &r.FilesChanged, &createdAt); err != nil {
		return nil, fmt.Errorf("scan agent run: %w", err)
	}
	var err error
	r.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func collectAgentRuns(rows *sql.Rows) ([]AgentRun, error) {
	var out []AgentRun
	for rows.Next() {
		r, err := scanAgentRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Agent Lessons
// ---------------------------------------------------------------------------

// CreateLesson inserts an agent lesson record.
func (d *DB) CreateLesson(ctx context.Context, executionID int64, agentType, lessonType, content string) error {
	ts := now()
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO agent_lessons (execution_id, agent_type, lesson_type, content, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		executionID, agentType, lessonType, content, ts,
	)
	if err != nil {
		return fmt.Errorf("create lesson: %w", err)
	}
	return nil
}

// ListLessons returns all lessons for a specific execution.
func (d *DB) ListLessons(ctx context.Context, executionID int64) ([]AgentLesson, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, execution_id, agent_type, lesson_type, content, created_at
		 FROM agent_lessons WHERE execution_id = ? ORDER BY created_at`,
		executionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list lessons: %w", err)
	}
	defer rows.Close()
	return collectLessons(rows)
}

// ListAllLessons returns all lessons across all executions in a cluster.
func (d *DB) ListAllLessons(ctx context.Context, clusterID int64) ([]AgentLesson, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT al.id, al.execution_id, al.agent_type, al.lesson_type, al.content, al.created_at
		 FROM agent_lessons al
		 JOIN executions e ON e.id = al.execution_id
		 WHERE e.cluster_id = ?
		 ORDER BY al.created_at`,
		clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list all lessons: %w", err)
	}
	defer rows.Close()
	return collectLessons(rows)
}

func scanLesson(s scanner) (*AgentLesson, error) {
	var l AgentLesson
	var createdAt string
	if err := s.Scan(&l.ID, &l.ExecutionID, &l.AgentType, &l.LessonType, &l.Content, &createdAt); err != nil {
		return nil, fmt.Errorf("scan lesson: %w", err)
	}
	var err error
	l.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func collectLessons(rows *sql.Rows) ([]AgentLesson, error) {
	var out []AgentLesson
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Context Search (for Commander reuse)
// ---------------------------------------------------------------------------

// ListRecentAgentRuns returns the most recent agent runs across all executions in a cluster.
// Used to give the Commander context about what work has recently been done.
func (d *DB) ListRecentAgentRuns(ctx context.Context, clusterID int64, limit int) ([]AgentRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.conn.QueryContext(ctx, `
		SELECT ar.id, ar.execution_id, ar.agent_type, ar.role, ar.prompt,
		       ar.summary, ar.outcome, ar.files_changed, ar.created_at
		FROM agent_runs ar
		JOIN executions e ON e.id = ar.execution_id
		WHERE e.cluster_id = ?
		ORDER BY ar.created_at DESC
		LIMIT ?
	`, clusterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent agent runs: %w", err)
	}
	defer rows.Close()
	return collectAgentRuns(rows)
}

// SearchRelevantRuns finds agent runs relevant to a thread by matching thread
// membership and keyword overlap on task prompts and agent summaries.
// Keywords are matched with LIKE and results are ranked by match count.
func (d *DB) SearchRelevantRuns(ctx context.Context, clusterID int64, threadID int64, keywords []string, limit int) ([]AgentRun, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	// Cap keywords to prevent excessive query complexity.
	if len(keywords) > 20 {
		keywords = keywords[:20]
	}
	if limit <= 0 {
		limit = 20
	}

	// Build a dynamic query that scores runs by keyword overlap.
	// Each keyword contributes +1 to the score if it appears in the task prompt or agent summary.
	// The score expression appears twice in the query (SELECT and WHERE), so keyword patterns
	// must be supplied for each occurrence.
	var scoreParts []string
	for range keywords {
		scoreParts = append(scoreParts,
			"(CASE WHEN t.prompt LIKE ? THEN 1 ELSE 0 END + CASE WHEN ar.summary LIKE ? THEN 1 ELSE 0 END)",
		)
	}
	scoreExpr := strings.Join(scoreParts, " + ")

	query := fmt.Sprintf(`
		SELECT ar.id, ar.execution_id, ar.agent_type, ar.role, ar.prompt,
		       ar.summary, ar.outcome, ar.files_changed, ar.created_at,
		       (%s) AS relevance
		FROM agent_runs ar
		JOIN executions e ON e.id = ar.execution_id
		JOIN tasks t ON t.id = e.task_id
		WHERE e.cluster_id = ?
		  AND (t.thread_id = ? OR (%s) > 0)
		ORDER BY relevance DESC, ar.created_at DESC
		LIMIT ?
	`, scoreExpr, scoreExpr)

	// Build args in the order they appear in the query:
	// 1. Keyword patterns for the SELECT score expression
	// 2. clusterID, threadID for the WHERE clause
	// 3. Keyword patterns again for the WHERE score expression
	// 4. limit
	var args []any
	for _, kw := range keywords {
		pattern := "%" + kw + "%"
		args = append(args, pattern, pattern)
	}
	args = append(args, clusterID, threadID)
	for _, kw := range keywords {
		pattern := "%" + kw + "%"
		args = append(args, pattern, pattern)
	}
	args = append(args, limit)

	rows, err := d.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search relevant runs: %w", err)
	}
	defer rows.Close()

	var out []AgentRun
	for rows.Next() {
		var r AgentRun
		var createdAt string
		var relevance int
		if err := rows.Scan(&r.ID, &r.ExecutionID, &r.AgentType, &r.Role,
			&r.Prompt, &r.Summary, &r.Outcome, &r.FilesChanged, &createdAt, &relevance); err != nil {
			return nil, fmt.Errorf("scan relevant run: %w", err)
		}
		r.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
