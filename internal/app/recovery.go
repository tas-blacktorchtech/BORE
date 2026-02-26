package app

import (
	"context"
	"fmt"

	"bore/internal/db"
)

// recoverInterrupted detects executions that were running when the app
// last crashed or was killed, and marks them as interrupted.
// TODO(v2): Also mark orphaned agent_runs (those without an outcome) as "failed"
// within the recovered executions. Currently only execution-level recovery is performed.
func (a *App) recoverInterrupted(ctx context.Context) error {
	if a.db == nil || a.cluster == nil {
		return fmt.Errorf("app: recovery requires an open cluster")
	}

	running, err := a.db.ListExecutionsByStatus(ctx, a.cluster.ID, db.StatusRunning)
	if err != nil {
		return fmt.Errorf("app: query running executions: %w", err)
	}

	for _, exec := range running {
		if err := a.db.UpdateExecutionStatus(ctx, exec.ID, db.StatusInterrupted); err != nil {
			return fmt.Errorf("app: mark execution %d interrupted: %w", exec.ID, err)
		}
		if err := a.db.UpdateTaskStatus(ctx, exec.TaskID, db.StatusInterrupted); err != nil {
			return fmt.Errorf("app: mark task %d interrupted: %w", exec.TaskID, err)
		}
		if a.logs != nil {
			a.logs.System.Warn("app: recovered interrupted execution id=%d task_id=%d", exec.ID, exec.TaskID)
		}
	}

	if len(running) > 0 && a.logs != nil {
		a.logs.System.Info("app: crash recovery marked %d execution(s) as interrupted", len(running))
	}

	return nil
}
