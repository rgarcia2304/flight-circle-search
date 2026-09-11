package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/riverqueue/river"
)

// retentionWindow is how long a search_jobs row is kept before cleanup.
const retentionWindow = 30 * 24 * time.Hour

// RetentionCleanupArgs is the River job args for the periodic retention sweep.
// It carries no fields — the cutoff is computed at run time, not scheduling time.
type RetentionCleanupArgs struct{}

func (RetentionCleanupArgs) Kind() string { return "RetentionCleanupArgs" }

type retentionRepo interface {
	DeleteJobsOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// RetentionWorker deletes search_jobs (and their cascaded search_job_results)
// older than retentionWindow. Nothing else in the system cleans this data up.
type RetentionWorker struct {
	river.WorkerDefaults[RetentionCleanupArgs]
	repo retentionRepo
}

func NewRetentionWorker(repo retentionRepo) *RetentionWorker {
	return &RetentionWorker{repo: repo}
}

func (w *RetentionWorker) Work(ctx context.Context, _ *river.Job[RetentionCleanupArgs]) error {
	cutoff := time.Now().Add(-retentionWindow)
	deleted, err := w.repo.DeleteJobsOlderThan(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("retention cleanup: %w", err)
	}
	log.Printf("retention cleanup: deleted %d search_jobs older than %s", deleted, cutoff.Format(time.RFC3339))
	return nil
}

// retentionJobArgsConstructor is the PeriodicJobConstructor for the retention
// sweep, split out from retentionPeriodicJob so it's directly testable.
func retentionJobArgsConstructor() (river.JobArgs, *river.InsertOpts) {
	return RetentionCleanupArgs{}, &river.InsertOpts{Queue: searchQueue}
}

// retentionPeriodicJob schedules the retention sweep to run once a day.
func retentionPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		retentionJobArgsConstructor,
		&river.PeriodicJobOpts{ID: "retention-cleanup"},
	)
}
