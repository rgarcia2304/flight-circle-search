package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

var (
	apiURL = flag.String("api-url", envOr("API_URL", "http://localhost:8080"),
		"Base URL for the job API (default: http://localhost:8080)")
	date        = flag.String("date", "", "departure date (YYYY-MM-DD) — use depart_from and depart_to for ranges")
	departFrom  = flag.String("depart-from", "", "start of date range (YYYY-MM-DD)")
	departTo    = flag.String("depart-to", "", "end of date range (YYYY-MM-DD)")
	originLat   = flag.Float64("origin-lat", 40.6413, "origin circle center latitude")
	originLng   = flag.Float64("origin-lng", -73.7781, "origin circle center longitude")
	originR     = flag.Float64("origin-r", 322, "origin circle radius in km")
	destLat     = flag.Float64("dest-lat", 52.3667, "destination circle center latitude")
	destLng     = flag.Float64("dest-lng", 13.5033, "destination circle center longitude")
	destR       = flag.Float64("dest-r", 644, "destination circle radius in km")
	pollInterval = flag.Duration("poll", 2*time.Second, "poll interval for job status")
	pollTimeout  = flag.Duration("timeout", 10*time.Minute, "max time to wait for job completion")
)

func main() {
	flag.Parse()

	if *date == "" && (*departFrom == "" || *departTo == "") {
		fmt.Println("Usage: livetest -api-url=http://localhost:8080 -date=YYYY-MM-DD [-depart-from=Y -depart-to=Z]")
		fmt.Println("  or: livetest -api-url=http://localhost:8080 -depart-from=YYYY-MM-DD -depart-to=YYYY-MM-DD")
		os.Exit(1)
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

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

type jobStatus struct {
	ID             string   `json:"id"`
	Status         string   `json:"status"`
	TotalPairs     int      `json:"total_pairs"`
	CompletedPairs int      `json:"completed_pairs"`
	FailedPairs    int      `json:"failed_pairs"`
	ErrorSummary   *string  `json:"error_summary,omitempty"`
	Results        []result `json:"results,omitempty"`
}

type result struct {
	Origin        string          `json:"origin"`
	Destination   string          `json:"destination"`
	DepartureDate string          `json:"departure_date"`
	Status        string          `json:"status"`
	Fare          json.RawMessage `json:"fare,omitempty"`
	Error         *string         `json:"error,omitempty"`
}

func run() error {
	from, to := dateRange()
	log.Printf("Submitting job: origin=(%.4f,%.4f)±%.0fkm dest=(%.4f,%.4f)±%.0fkm dates=%s..%s",
		*originLat, *originLng, *originR, *destLat, *destLng, *destR, from, to)

	jobID, total, err := submitJob(from, to)
	if err != nil {
		return fmt.Errorf("submit: %w", err)
	}
	log.Printf("Job submitted: id=%s pairs=%d", jobID, total)

	ctx, cancel := context.WithTimeout(context.Background(), *pollTimeout)
	defer cancel()

	job, err := pollJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("poll: %w", err)
	}

	printResults(job)
	return nil
}

func dateRange() (from, to string) {
	if *date != "" {
		return *date, *date
	}
	return *departFrom, *departTo
}

func submitJob(from, to string) (string, int, error) {
	body, _ := json.Marshal(submitRequest{
		OriginLat:  *originLat,
		OriginLng:  *originLng,
		OriginR:    *originR,
		DestLat:    *destLat,
		DestLng:    *destLng,
		DestR:      *destR,
		DepartFrom: from,
		DepartTo:   to,
	})
	resp, err := http.Post(*apiURL+"/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return "", 0, fmt.Errorf("submit failed (%d): %s", resp.StatusCode, data)
	}
	var r submitResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return "", 0, fmt.Errorf("decode response: %w", err)
	}
	return r.JobID, r.TotalPairs, nil
}

func pollJob(ctx context.Context, jobID string) (*jobStatus, error) {
	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()
	url := fmt.Sprintf("%s/jobs/%s?include=results", *apiURL, jobID)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for job: %w", ctx.Err())
		case <-ticker.C:
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("poll error: %v", err)
			continue
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("job not found")
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("poll status %d: %s", resp.StatusCode, data)
			continue
		}

		var job jobStatus
		if err := json.Unmarshal(data, &job); err != nil {
			log.Printf("decode error: %v", err)
			continue
		}
		if job.Status == "complete" || job.Status == "failed" {
			return &job, nil
		}
		log.Printf("Job %s: status=%s completed=%d/%d failed=%d",
			jobID, job.Status, job.CompletedPairs, job.TotalPairs, job.FailedPairs)
	}
}

type fare struct {
	Origin  string
	Dest    string
	Price   int
	Airline string
	Flight  string
	Date    string
	Duration int
}

func printResults(job *jobStatus) {
	fmt.Printf("\n=== Job %s (%s) ===\n", job.ID, job.Status)
	fmt.Printf("Pairs: %d completed, %d failed out of %d total\n\n",
		job.CompletedPairs, job.FailedPairs, job.TotalPairs)

	var fares []fare
	for _, r := range job.Results {
		if r.Status != "complete" || len(r.Fare) == 0 {
			continue
		}
		var faresForPair []map[string]any
		if err := json.Unmarshal(r.Fare, &faresForPair); err != nil {
			continue
		}
		for _, f := range faresForPair {
			if price, ok := f["price"].(float64); ok {
				fares = append(fares, fare{
					Origin:  r.Origin,
					Dest:    r.Destination,
					Price:   int(price * 100),
					Airline: str(f["airline"]),
					Flight:  str(f["flight_number"]),
					Date:    r.DepartureDate,
					Duration: intf(f["duration"]),
				})
			}
		}
	}

	sort.Slice(fares, func(i, j int) bool { return fares[i].Price < fares[j].Price })

	if len(fares) == 0 {
		fmt.Println("No fares found.")
		return
	}

	fmt.Printf("%-3s %-7s %-12s %-6s %-8s %-12s %s\n",
		"#", "PRICE", "ROUTE", "AIRLINE", "FLIGHT", "DEP_DATE", "DURATION")
	fmt.Println(strings.Repeat("-", 75))
	for i, f := range fares {
		if i >= 20 {
			fmt.Printf("... and %d more results\n", len(fares)-i)
			break
		}
		fmt.Printf("%-3d $%-6.2f %s→%s %-6s %-8s %-12s %d min\n",
			i+1, float64(f.Price)/100,
			f.Origin, f.Dest,
			f.Airline, f.Flight, f.Date, f.Duration)
	}
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func intf(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
