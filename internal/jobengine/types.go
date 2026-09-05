package jobengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	StatusPending  JobStatus = "pending"
	StatusRunning JobStatus = "running"
	StatusComplete JobStatus = "complete"
	StatusFailed  JobStatus = "failed"
)

var (
	ErrTooManyPairs  = errors.New("too many pairs")
	ErrInvalidInput = errors.New("invalid input")
)

type SearchJob struct {
	ID             uuid.UUID
	Status         JobStatus
	SubmittedAt    time.Time
	CompletedAt    *time.Time
	TotalPairs     int
	CompletedPairs int
	FailedPairs    int
	ErrorSummary   *string
	Request        SearchRequest
}

type SearchRequest struct {
	OriginLat  float64 `json:"origin_lat"`
	OriginLng  float64 `json:"origin_lng"`
	OriginR    float64 `json:"origin_r"`
	DestLat    float64 `json:"dest_lat"`
	DestLng    float64 `json:"dest_lng"`
	DestR      float64 `json:"dest_r"`
	DepartFrom string  `json:"depart_from"`
	DepartTo   string  `json:"depart_to"`
}

type SearchJobResult struct {
	ID            int64
	JobID         uuid.UUID
	Origin        string
	Destination   string
	DepartureDate string
	Status        string
	Fare          *json.RawMessage
	Error         *string
	CompletedAt   *time.Time
}

const MaxPairsPerJob = 5000

var dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validateRequest(req SearchRequest) error {
	if req.OriginLat < -90 || req.OriginLat > 90 {
		return fmt.Errorf("origin_lat: %w", ErrInvalidInput)
	}
	if req.OriginLng < -180 || req.OriginLng > 180 {
		return fmt.Errorf("origin_lng: %w", ErrInvalidInput)
	}
	if req.DestLat < -90 || req.DestLat > 90 {
		return fmt.Errorf("dest_lat: %w", ErrInvalidInput)
	}
	if req.DestLng < -180 || req.DestLng > 180 {
		return fmt.Errorf("dest_lng: %w", ErrInvalidInput)
	}
	if req.OriginR <= 0 || req.OriginR > 20000 {
		return fmt.Errorf("origin_r: %w", ErrInvalidInput)
	}
	if req.DestR <= 0 || req.DestR > 20000 {
		return fmt.Errorf("dest_r: %w", ErrInvalidInput)
	}
	if !dateRegex.MatchString(req.DepartFrom) {
		return fmt.Errorf("depart_from: %w", ErrInvalidInput)
	}
	if !dateRegex.MatchString(req.DepartTo) {
		return fmt.Errorf("depart_to: %w", ErrInvalidInput)
	}
	if _, err := time.Parse("2006-01-02", req.DepartFrom); err != nil {
		return fmt.Errorf("depart_from: %w", ErrInvalidInput)
	}
	if _, err := time.Parse("2006-01-02", req.DepartTo); err != nil {
		return fmt.Errorf("depart_to: %w", ErrInvalidInput)
	}
	return nil
}
