package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
)

type mockRetentionRepo struct {
	gotCutoff time.Time
	deleted   int64
	err       error
}

func (m *mockRetentionRepo) DeleteJobsOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	m.gotCutoff = cutoff
	return m.deleted, m.err
}

func TestRetentionWorker_Work(t *testing.T) {
	repo := &mockRetentionRepo{deleted: 3}
	w := NewRetentionWorker(repo)

	before := time.Now()
	err := w.Work(context.Background(), &river.Job[RetentionCleanupArgs]{Args: RetentionCleanupArgs{}})
	after := time.Now()
	if err != nil {
		t.Fatalf("Work returned error: %v", err)
	}

	wantMin := before.Add(-retentionWindow)
	wantMax := after.Add(-retentionWindow)
	if repo.gotCutoff.Before(wantMin) || repo.gotCutoff.After(wantMax) {
		t.Errorf("cutoff = %v, want between %v and %v", repo.gotCutoff, wantMin, wantMax)
	}
}

func TestRetentionWorker_Work_PropagatesError(t *testing.T) {
	repo := &mockRetentionRepo{err: errors.New("db down")}
	w := NewRetentionWorker(repo)

	err := w.Work(context.Background(), &river.Job[RetentionCleanupArgs]{Args: RetentionCleanupArgs{}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRetentionJobArgsConstructor_UsesSearchQueue(t *testing.T) {
	args, opts := retentionJobArgsConstructor()
	if _, ok := args.(RetentionCleanupArgs); !ok {
		t.Fatalf("constructor returned %T, want RetentionCleanupArgs", args)
	}
	if opts == nil || opts.Queue != searchQueue {
		t.Fatalf("opts.Queue = %+v, want %q", opts, searchQueue)
	}
}

func TestRetentionPeriodicJob_NotNil(t *testing.T) {
	if retentionPeriodicJob() == nil {
		t.Fatal("retentionPeriodicJob() returned nil")
	}
}
