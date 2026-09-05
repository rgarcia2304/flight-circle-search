CREATE TABLE search_jobs (
    id UUID PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'pending',
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    total_pairs INT NOT NULL,
    completed_pairs INT NOT NULL DEFAULT 0,
    failed_pairs INT NOT NULL DEFAULT 0,
    error_summary TEXT,
    request JSONB NOT NULL
);

CREATE INDEX idx_search_jobs_status ON search_jobs(status);
CREATE INDEX idx_search_jobs_submitted_at ON search_jobs(submitted_at);

CREATE TABLE search_job_results (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES search_jobs(id) ON DELETE CASCADE,
    origin TEXT NOT NULL,
    destination TEXT NOT NULL,
    departure_date DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    fare JSONB,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_search_job_results_job_id ON search_job_results(job_id);
CREATE INDEX idx_search_job_results_status ON search_job_results(status);
