package worker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/rgarcia2304/flight-circle-search/internal/fareprovider"
	"github.com/riverqueue/river"
)

// --- mock repository ---

type mockRepo struct {
	mu             sync.Mutex
	results        map[int64]resultRecord
	completeIfDone bool
}

type resultRecord struct {
	status string
	fare   json.RawMessage
	err    *string
}

func newMockRepo() *mockRepo {
	return &mockRepo{results: make(map[int64]resultRecord)}
}

func (m *mockRepo) UpdateResult(_ context.Context, id int64, status string, fare json.RawMessage, errMsg *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[id] = resultRecord{status: status, fare: fare, err: errMsg}
	return nil
}

func (m *mockRepo) MarkResultDeadLetter(_ context.Context, id int64, errMsg string) error {
	return m.UpdateResult(context.Background(), id, "dead_letter", nil, &errMsg)
}

func (m *mockRepo) UpdateJobProgress(_ context.Context, _ uuid.UUID) error { return nil }

func (m *mockRepo) MarkJobCompleteIfDone(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.completeIfDone, nil
}

// --- mock fare provider ---

type mockProvider struct {
	fn func(ctx context.Context, req fareprovider.SearchRequest) ([]fareprovider.Fare, error)
}

func (m *mockProvider) Search(ctx context.Context, req fareprovider.SearchRequest) ([]fareprovider.Fare, error) {
	return m.fn(ctx, req)
}

// --- helpers ---

func newJob(args SearchJobArgs) *river.Job[SearchJobArgs] {
	return &river.Job[SearchJobArgs]{Args: args}
}

// --- tests ---

func TestFareWorker_SuccessPersistsResult(t *testing.T) {
	repo := newMockRepo()
	provider := &mockProvider{
		fn: func(_ context.Context, _ fareprovider.SearchRequest) ([]fareprovider.Fare, error) {
			return []fareprovider.Fare{{
				Origin: "JFK", Destination: "LHR",
				Price: 30300, Currency: "USD",
			}}, nil
		},
	}
	rl := NewRateLimiter(1000, 10)
	w := NewFareWorker(repo, provider, rl)

	job := newJob(SearchJobArgs{
		ResultID: 42, JobID: uuid.New(),
		Origin: "JFK", Destination: "LHR", Date: "2026-10-15",
	})
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	rec, ok := repo.results[42]
	if !ok {
		t.Fatal("result not persisted")
	}
	if rec.status != "complete" {
		t.Errorf("status = %q, want complete", rec.status)
	}
	if len(rec.fare) == 0 {
		t.Error("fare not stored")
	}
}

func TestFareWorker_AuthErrorDeadLetters(t *testing.T) {
	repo := newMockRepo()
	provider := &mockProvider{
		fn: func(_ context.Context, _ fareprovider.SearchRequest) ([]fareprovider.Fare, error) {
			return nil, fareprovider.ErrAuth
		},
	}
	rl := NewRateLimiter(1000, 10)
	w := NewFareWorker(repo, provider, rl)

	job := newJob(SearchJobArgs{
		ResultID: 7, JobID: uuid.New(),
		Origin: "JFK", Destination: "LHR", Date: "2026-10-15",
	})
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v, want nil (auth should be terminal)", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	rec, ok := repo.results[7]
	if !ok {
		t.Fatal("result not persisted")
	}
	if rec.status != "dead_letter" {
		t.Errorf("status = %q, want dead_letter", rec.status)
	}
	if rec.err == nil || *rec.err != "auth_failed" {
		t.Errorf("err = %v, want auth_failed", rec.err)
	}
}

func TestFareWorker_RateLimitedRetries(t *testing.T) {
	repo := newMockRepo()
	provider := &mockProvider{
		fn: func(_ context.Context, _ fareprovider.SearchRequest) ([]fareprovider.Fare, error) {
			return nil, fareprovider.ErrRateLimited
		},
	}
	rl := NewRateLimiter(1000, 10)
	w := NewFareWorker(repo, provider, rl)

	job := newJob(SearchJobArgs{
		ResultID: 99, JobID: uuid.New(),
		Origin: "JFK", Destination: "LHR", Date: "2026-10-15",
	})
	err := w.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error to trigger retry, got nil")
	}
	if !errors.Is(err, fareprovider.ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestFareWorker_ProviderDownRetries(t *testing.T) {
	repo := newMockRepo()
	provider := &mockProvider{
		fn: func(_ context.Context, _ fareprovider.SearchRequest) ([]fareprovider.Fare, error) {
			return nil, fareprovider.ErrProviderDown
		},
	}
	rl := NewRateLimiter(1000, 10)
	w := NewFareWorker(repo, provider, rl)

	job := newJob(SearchJobArgs{
		ResultID: 99, JobID: uuid.New(),
		Origin: "JFK", Destination: "LHR", Date: "2026-10-15",
	})
	err := w.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, fareprovider.ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown", err)
	}
}

func TestFareWorker_EmptyResultsStillComplete(t *testing.T) {
	repo := newMockRepo()
	provider := &mockProvider{
		fn: func(_ context.Context, _ fareprovider.SearchRequest) ([]fareprovider.Fare, error) {
			return []fareprovider.Fare{}, nil
		},
	}
	rl := NewRateLimiter(1000, 10)
	w := NewFareWorker(repo, provider, rl)

	job := newJob(SearchJobArgs{
		ResultID: 5, JobID: uuid.New(),
		Origin: "JFK", Destination: "XYZ", Date: "2026-10-15",
	})
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	rec := repo.results[5]
	if rec.status != "complete" {
		t.Errorf("status = %q, want complete", rec.status)
	}
}


