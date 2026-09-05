package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/rgarcia2304/flight-circle-search/internal/fareprovider"
	"github.com/riverqueue/river"
)

// SearchJobArgs is the River job args for a single fare search.
type SearchJobArgs struct {
	ResultID    int64     `json:"result_id"`
	JobID       uuid.UUID `json:"job_id"`
	Origin      string    `json:"origin"`
	Destination string    `json:"destination"`
	Date        string    `json:"date"`
}

// Kind returns the River job kind — matches what we enqueue.
func (SearchJobArgs) Kind() string { return "SearchJobArgs" }

// Work is the River handler. It fetches fares for one (origin, destination, date) tuple.
type FareWorker struct {
	river.WorkerDefaults[SearchJobArgs]
	repo         fareSearchRepo
	fareProvider fareprovider.FareProvider
	rateLimiter  *RateLimiter
}

type fareSearchRepo interface {
	UpdateResult(ctx context.Context, id int64, status string, fare json.RawMessage, errMsg *string) error
	MarkResultDeadLetter(ctx context.Context, id int64, errMsg string) error
	UpdateJobProgress(ctx context.Context, id uuid.UUID) error
	MarkJobCompleteIfDone(ctx context.Context, id uuid.UUID) (bool, error)
}

func NewFareWorker(repo fareSearchRepo, fp fareprovider.FareProvider, rl *RateLimiter) *FareWorker {
	return &FareWorker{
		repo:         repo,
		fareProvider: fp,
		rateLimiter: rl,
	}
}

func (w *FareWorker) Work(ctx context.Context, job *river.Job[SearchJobArgs]) error {
	args := job.Args

	if err := w.rateLimiter.Wait(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return river.JobSnooze(0)
		}
		return fmt.Errorf("rate limiter wait: %w", err)
	}
	defer w.rateLimiter.Release()

	fares, err := w.fareProvider.Search(ctx, fareprovider.SearchRequest{
		Origin:      args.Origin,
		Destination: args.Destination,
		Date:        args.Date,
	})

	if err != nil {
		return w.handleError(ctx, args.ResultID, args.JobID, err)
	}

	var fareBytes json.RawMessage
	if len(fares) > 0 {
		b, merr := json.Marshal(fares)
		if merr != nil {
			return fmt.Errorf("marshal fares: %w", merr)
		}
		fareBytes = b
	}

	if err := w.repo.UpdateResult(ctx, args.ResultID, "complete", fareBytes, nil); err != nil {
		log.Printf("failed to persist result for result_id=%d: %v", args.ResultID, err)
		return fmt.Errorf("update result: %w", err)
	}

	if len(fares) == 0 {
		log.Printf("[fare] %s→%s %s: no fares available", args.Origin, args.Destination, args.Date)
	} else {
		w.logBestFare(args.Origin, args.Destination, args.Date, fares)
	}

	if err := w.repo.UpdateJobProgress(ctx, args.JobID); err != nil {
		log.Printf("failed to update job progress for job_id=%s: %v", args.JobID, err)
	}

	if _, err := w.repo.MarkJobCompleteIfDone(ctx, args.JobID); err != nil {
		log.Printf("failed to mark job complete for job_id=%s: %v", args.JobID, err)
	}

	return nil
}

func (w *FareWorker) logBestFare(origin, dest, date string, fares []fareprovider.Fare) {
	best := fares[0]
	price := float64(best.Price) / 100
	var transferStr string
	if best.Transfers == 0 {
		transferStr = "direct"
	} else {
		transferStr = fmt.Sprintf("%d stop(s)", best.Transfers)
	}
	log.Printf("[fare] %s→%s %s | %s %s %s | %s | %s | %s | $%.2f %s | %s",
		origin, dest, date,
		best.Airline, best.FlightNumber,
		best.DepartureAt.Format(time.RFC822),
		best.OriginAirport+"→"+best.DestinationAirport,
		transferStr,
		best.Duration.Round(time.Minute).String(),
		price, best.Currency,
		best.Link,
	)
}

func (w *FareWorker) handleError(ctx context.Context, resultID int64, jobID uuid.UUID, err error) error {
	switch {
	case errors.Is(err, fareprovider.ErrAuth):
		// Terminal — don't retry. Mark dead-letter.
		if err := w.markDeadLetter(ctx, resultID, jobID, "auth_failed"); err != nil {
			return err
		}
		return nil

	case errors.Is(err, fareprovider.ErrInvalidInput):
		// Should not happen — input was validated at submit time. Dead-letter.
		if err := w.markDeadLetter(ctx, resultID, jobID, "invalid_input"); err != nil {
			return err
		}
		return nil

	case errors.Is(err, fareprovider.ErrRateLimited):
		// Retryable — return error so River re-queues with exponential backoff.
		return err

	case errors.Is(err, fareprovider.ErrProviderDown):
		// Retryable — provider is temporarily down.
		return err

	default:
		// Unknown error — dead-letter to avoid infinite retry loop.
		if err := w.markDeadLetter(ctx, resultID, jobID, "unknown_error"); err != nil {
			return err
		}
		return nil
	}
}

func (w *FareWorker) markDeadLetter(ctx context.Context, resultID int64, jobID uuid.UUID, reason string) error {
	if err := w.repo.MarkResultDeadLetter(ctx, resultID, reason); err != nil {
		log.Printf("failed to dead-letter result_id=%d: %v", resultID, err)
		return err
	}
	if err := w.repo.UpdateJobProgress(ctx, jobID); err != nil {
		log.Printf("failed to update progress for job_id=%s: %v", jobID, err)
		return err
	}
	if _, err := w.repo.MarkJobCompleteIfDone(ctx, jobID); err != nil {
		log.Printf("failed to mark job complete for job_id=%s: %v", jobID, err)
		return err
	}
	return nil
}
