package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rgarcia2304/flight-circle-search/internal/jobengine"
	"github.com/rgarcia2304/flight-circle-search/internal/worker"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const (
	defaultQueueName   = "search"
	defaultMaxAttempts = 5
)

// riverEnqueuer adapts the River client to jobengine.Enqueuer.
type riverEnqueuer struct {
	client *river.Client[pgx.Tx]
}

func (r *riverEnqueuer) EnqueueBatch(ctx context.Context, results []jobengine.SearchJobResult) error {
	if len(results) == 0 {
		return nil
	}
	inserts := make([]river.InsertManyParams, 0, len(results))
	for _, res := range results {
		inserts = append(inserts, river.InsertManyParams{
			Args: worker.SearchJobArgs{
				ResultID:    res.ID,
				JobID:       res.JobID,
				Origin:      res.Origin,
				Destination: res.Destination,
				Date:        res.DepartureDate,
			},
			InsertOpts: &river.InsertOpts{
				MaxAttempts: defaultMaxAttempts,
				Queue:       defaultQueueName,
			},
		})
	}
	_, err := r.client.InsertMany(ctx, inserts)
	return err
}

// newRiverClient builds an insert-only River client: it registers a
// placeholder worker (River requires at least one) but is never Start()ed,
// so it only ever inserts jobs — the real fetch loop lives in cmd/worker.
func newRiverClient(pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &idleWorker{})

	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{defaultQueueName: {MaxWorkers: 1}},
		Workers: workers,
		Schema:  "public",
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return c, nil
}

type idleWorker struct {
	river.WorkerDefaults[worker.SearchJobArgs]
}

func (w *idleWorker) Work(_ context.Context, _ *river.Job[worker.SearchJobArgs]) error { return nil }
