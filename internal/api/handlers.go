package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rgarcia2304/flight-circle-search/internal/jobengine"
)

type submitRequest struct {
	OriginLat  float64 `json:"origin_lat"`
	OriginLng  float64 `json:"origin_lng"`
	OriginR    float64 `json:"origin_r"`
	DestLat    float64 `json:"dest_lat"`
	DestLng    float64 `json:"dest_lng"`
	DestR      float64 `json:"dest_r"`
	DepartFrom string  `json:"depart_from"`
	DepartTo   string  `json:"depart_to"`
}

type submitResponse struct {
	JobID      string `json:"job_id"`
	TotalPairs int    `json:"total_pairs"`
}

type jobResponse struct {
	ID             string           `json:"id"`
	Status         string           `json:"status"`
	SubmittedAt    time.Time        `json:"submitted_at"`
	CompletedAt    *time.Time       `json:"completed_at,omitempty"`
	TotalPairs     int              `json:"total_pairs"`
	CompletedPairs int              `json:"completed_pairs"`
	FailedPairs    int              `json:"failed_pairs"`
	ErrorSummary   *string          `json:"error_summary,omitempty"`
	Request        submitRequest    `json:"request"`
	Results        []resultResponse `json:"results,omitempty"`
}

type resultResponse struct {
	Origin        string          `json:"origin"`
	Destination   string          `json:"destination"`
	DepartureDate string          `json:"departure_date"`
	Status        string          `json:"status"`
	Fare          json.RawMessage `json:"fare,omitempty"`
	Error         *string         `json:"error,omitempty"`
}

func makeSubmitHandler(svc *jobengine.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req submitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		jobID, total, err := svc.Submit(r.Context(), jobengine.SearchRequest{
			OriginLat:  req.OriginLat,
			OriginLng:  req.OriginLng,
			OriginR:    req.OriginR,
			DestLat:    req.DestLat,
			DestLng:    req.DestLng,
			DestR:      req.DestR,
			DepartFrom: req.DepartFrom,
			DepartTo:   req.DepartTo,
		})
		if err != nil {
			switch {
			case errors.Is(err, jobengine.ErrTooManyPairs):
				writeError(w, http.StatusRequestEntityTooLarge, "too_many_pairs", err.Error())
			case errors.Is(err, jobengine.ErrInvalidInput):
				writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "internal", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusAccepted, submitResponse{
			JobID:      jobID.String(),
			TotalPairs: total,
		})
	}
}

func makeGetHandler(repo jobengine.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "job id must be a UUID")
			return
		}
		job, err := repo.GetJob(r.Context(), id)
		if err != nil {
			if errors.Is(err, jobengine.ErrJobNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "job not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}

		resp := jobResponse{
			ID:             job.ID.String(),
			Status:         string(job.Status),
			SubmittedAt:    job.SubmittedAt,
			CompletedAt:    job.CompletedAt,
			TotalPairs:     job.TotalPairs,
			CompletedPairs: job.CompletedPairs,
			FailedPairs:    job.FailedPairs,
			ErrorSummary:   job.ErrorSummary,
		}
		resp.Request = submitRequest{
			OriginLat:  job.Request.OriginLat,
			OriginLng:  job.Request.OriginLng,
			OriginR:    job.Request.OriginR,
			DestLat:    job.Request.DestLat,
			DestLng:    job.Request.DestLng,
			DestR:      job.Request.DestR,
			DepartFrom: job.Request.DepartFrom,
			DepartTo:   job.Request.DepartTo,
		}

		if r.URL.Query().Get("include") == "results" {
			results, err := repo.ListResults(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			for _, res := range results {
				rr := resultResponse{
					Origin:        res.Origin,
					Destination:   res.Destination,
					DepartureDate: res.DepartureDate,
					Status:        res.Status,
					Error:         res.Error,
				}
				if res.Fare != nil {
					rr.Fare = *res.Fare
				}
				resp.Results = append(resp.Results, rr)
			}
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}
