package jobengine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

// Repository persists search jobs and their per-pair results.
type Repository interface {
	CreateJob(ctx context.Context, job SearchJob, pairs []SearchJobResult) (uuid.UUID, error)
	GetJob(ctx context.Context, id uuid.UUID) (SearchJob, error)
	ListResults(ctx context.Context, jobID uuid.UUID) ([]SearchJobResult, error)
	GetPendingResults(ctx context.Context, jobID uuid.UUID) ([]SearchJobResult, error)
	ClaimNextResult(ctx context.Context, jobID uuid.UUID) (*SearchJobResult, error)
	UpdateResult(ctx context.Context, id int64, status string, fare json.RawMessage, errMsg *string) error
	MarkResultDeadLetter(ctx context.Context, id int64, errMsg string) error
	UpdateJobProgress(ctx context.Context, id uuid.UUID) error
	MarkJobStatus(ctx context.Context, id uuid.UUID, status JobStatus, errSummary *string) error
	MarkJobCompleteIfDone(ctx context.Context, id uuid.UUID) (bool, error)
}

// PostgresRepository implements Repository against Postgres.
type PostgresRepository struct {
	db QueryRower
}

// QueryRower is the subset of pgxpool we need. Allows mocking in tests.
type QueryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewPostgresRepository(db QueryRower) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateJob creates the parent job and inserts all per-pair result rows in a single tx.
func (r *PostgresRepository) CreateJob(ctx context.Context, job SearchJob, pairs []SearchJobResult) (uuid.UUID, error) {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}

	reqJSON, err := json.Marshal(job.Request)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal request: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO search_jobs (id, status, total_pairs, request)
		VALUES ($1, $2, $3, $4)
	`, job.ID, string(StatusPending), job.TotalPairs, reqJSON); err != nil {
		return uuid.Nil, fmt.Errorf("insert job: %w", err)
	}

	if len(pairs) > 0 {
		rows := make([][]any, 0, len(pairs))
		for _, p := range pairs {
			rows = append(rows, []any{
				job.ID,
				p.Origin,
				p.Destination,
				p.DepartureDate,
				"pending",
			})
		}
		// Use COPY for efficient batch insert.
		_, err = tx.CopyFrom(ctx,
			pgx.Identifier{"search_job_results"},
			[]string{"job_id", "origin", "destination", "departure_date", "status"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("copy result rows: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return job.ID, nil
}

func (r *PostgresRepository) GetJob(ctx context.Context, id uuid.UUID) (SearchJob, error) {
	var job SearchJob
	var reqBytes []byte
	var completedAt sql.NullTime
	var errSummary sql.NullString

	err := r.db.QueryRow(ctx, `
		SELECT id, status, submitted_at, completed_at, total_pairs,
		       completed_pairs, failed_pairs, error_summary, request
		FROM search_jobs
		WHERE id = $1
	`, id).Scan(
		&job.ID, &job.Status, &job.SubmittedAt, &completedAt,
		&job.TotalPairs, &job.CompletedPairs, &job.FailedPairs,
		&errSummary, &reqBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SearchJob{}, ErrJobNotFound
		}
		return SearchJob{}, fmt.Errorf("select job: %w", err)
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	if errSummary.Valid {
		job.ErrorSummary = &errSummary.String
	}
	if err := json.Unmarshal(reqBytes, &job.Request); err != nil {
		return SearchJob{}, fmt.Errorf("unmarshal request: %w", err)
	}
	return job, nil
}

func (r *PostgresRepository) GetPendingResults(ctx context.Context, jobID uuid.UUID) ([]SearchJobResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, job_id, origin, destination, departure_date,
		       status, fare, error, completed_at
		FROM search_job_results
		WHERE job_id = $1 AND status = 'pending'
		ORDER BY id
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("query pending results: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

func (r *PostgresRepository) ListResults(ctx context.Context, jobID uuid.UUID) ([]SearchJobResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, job_id, origin, destination, departure_date,
		       status, fare, error, completed_at
		FROM search_job_results
		WHERE job_id = $1
		ORDER BY id
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("query results: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

func scanResults(rows pgx.Rows) ([]SearchJobResult, error) {
	var out []SearchJobResult
	for rows.Next() {
		var r SearchJobResult
		var fareBytes []byte
		var errMsg sql.NullString
		var completedAt sql.NullTime
		var departDate pgtype.Date
		if err := rows.Scan(
			&r.ID, &r.JobID, &r.Origin, &r.Destination, &departDate,
			&r.Status, &fareBytes, &errMsg, &completedAt,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if departDate.Valid {
			r.DepartureDate = departDate.Time.Format("2006-01-02")
		}
		if len(fareBytes) > 0 {
			rm := json.RawMessage(fareBytes)
			r.Fare = &rm
		}
		if errMsg.Valid {
			r.Error = &errMsg.String
		}
		if completedAt.Valid {
			r.CompletedAt = &completedAt.Time
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClaimNextResult atomically claims a pending result for this job.
// Returns nil if no pending results remain.
func (r *PostgresRepository) ClaimNextResult(ctx context.Context, jobID uuid.UUID) (*SearchJobResult, error) {
	var res SearchJobResult
	var fareBytes []byte
	var errMsg sql.NullString
	var completedAt sql.NullTime
	var departDate pgtype.Date

	err := r.db.QueryRow(ctx, `
		UPDATE search_job_results
		SET status = 'claimed', completed_at = now()
		WHERE id = (
			SELECT id FROM search_job_results
			WHERE job_id = $1 AND status = 'pending'
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, job_id, origin, destination, departure_date,
		          status, fare, error, completed_at
	`, jobID).Scan(
		&res.ID, &res.JobID, &res.Origin, &res.Destination, &departDate,
		&res.Status, &fareBytes, &errMsg, &completedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("claim: %w", err)
	}
	if departDate.Valid {
		res.DepartureDate = departDate.Time.Format("2006-01-02")
	}
	if len(fareBytes) > 0 {
		rm := json.RawMessage(fareBytes)
		res.Fare = &rm
	}
	if errMsg.Valid {
		res.Error = &errMsg.String
	}
	if completedAt.Valid {
		res.CompletedAt = &completedAt.Time
	}
	return &res, nil
}

func (r *PostgresRepository) UpdateResult(ctx context.Context, id int64, status string, fare json.RawMessage, errMsg *string) error {
	var fareArg any
	if len(fare) > 0 {
		fareArg = string(fare)
	}
	var errArg any
	if errMsg != nil {
		errArg = *errMsg
	}
	_, err := r.db.Exec(ctx, `
		UPDATE search_job_results
		SET status = $1, fare = $2, error = $3, completed_at = now()
		WHERE id = $4
	`, status, fareArg, errArg, id)
	if err != nil {
		return fmt.Errorf("update result: %w", err)
	}
	return nil
}

func (r *PostgresRepository) MarkResultDeadLetter(ctx context.Context, id int64, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE search_job_results
		SET status = 'dead_letter', error = $1, completed_at = now()
		WHERE id = $2
	`, errMsg, id)
	if err != nil {
		return fmt.Errorf("mark dead-letter: %w", err)
	}
	return nil
}

// UpdateJobProgress recomputes completed/failed counts and updates parent job status.
func (r *PostgresRepository) UpdateJobProgress(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		WITH counts AS (
			SELECT
				COUNT(*) FILTER (WHERE status IN ('complete','failed','dead_letter')) AS done,
				COUNT(*) FILTER (WHERE status = 'dead_letter') AS dead,
				COUNT(*) FILTER (WHERE status = 'failed') AS failed
			FROM search_job_results WHERE job_id = $1
		)
		UPDATE search_jobs sj
		SET completed_pairs = c.done,
		    failed_pairs = c.dead + c.failed
		FROM counts c
		WHERE sj.id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	return nil
}

// MarkJobCompleteIfDone marks the job as complete if no pending or in-flight
// result rows remain. Returns true if the job was marked complete.
func (r *PostgresRepository) MarkJobCompleteIfDone(ctx context.Context, id uuid.UUID) (bool, error) {
	var remaining int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM search_job_results
		WHERE job_id = $1 AND status IN ('pending', 'claimed')
	`, id).Scan(&remaining); err != nil {
		return false, fmt.Errorf("count remaining: %w", err)
	}
	if remaining > 0 {
		return false, nil
	}

	var totalPairs, completedPairs, failedPairs int
	if err := r.db.QueryRow(ctx, `
		SELECT total_pairs, completed_pairs, failed_pairs FROM search_jobs WHERE id = $1
	`, id).Scan(&totalPairs, &completedPairs, &failedPairs); err != nil {
		return false, fmt.Errorf("read job: %w", err)
	}

	if completedPairs+failedPairs < totalPairs {
		return false, nil
	}

	finalStatus := StatusComplete
	if failedPairs == totalPairs {
		finalStatus = StatusFailed
	}

	// Finalize from either 'pending' or 'running': a job whose post-enqueue
	// MarkJobStatus(Running) call itself failed is stuck at 'pending' even
	// though its rows were successfully enqueued and are now all done —
	// gating on 'running' alone left that job permanently unfinalized. The
	// IN-list still makes this idempotent: once status becomes 'complete' or
	// 'failed' it no longer matches, so a racing second call is a no-op.
	tag, err := r.db.Exec(ctx, `
		UPDATE search_jobs
		SET status = $1, completed_at = now()
		WHERE id = $2 AND status IN ('pending', 'running')
	`, string(finalStatus), id)
	if err != nil {
		return false, fmt.Errorf("mark complete: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PostgresRepository) MarkJobStatus(ctx context.Context, id uuid.UUID, status JobStatus, errSummary *string) error {
	var errArg any
	if errSummary != nil {
		errArg = *errSummary
	}
	var completedAt any
	if status == StatusComplete || status == StatusFailed {
		completedAt = time.Now()
	}
	_, err := r.db.Exec(ctx, `
		UPDATE search_jobs
		SET status = $1, error_summary = $2, completed_at = $3
		WHERE id = $4
	`, string(status), errArg, completedAt, id)
	if err != nil {
		return fmt.Errorf("mark status: %w", err)
	}
	return nil
}

// EnqueueJob is a no-op kept for interface compatibility.
// River enqueues jobs directly via the worker package.
func (r *PostgresRepository) EnqueueJob(_ context.Context, _ uuid.UUID) error {
	return nil
}

// DeleteJobsOlderThan removes search_jobs (and, via ON DELETE CASCADE, their
// search_job_results) submitted before cutoff. Returns the number of jobs deleted.
func (r *PostgresRepository) DeleteJobsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM search_jobs WHERE submitted_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old jobs: %w", err)
	}
	return tag.RowsAffected(), nil
}
