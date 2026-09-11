package fareprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// GoogleFlightsScraper is a FareProvider backed by a local Python script
// (scripts/fare-scrape-poc/query_json.py, not committed to the repo — see
// its README) that in turn uses the fast-flights library to query Google
// Flights directly.
//
// LOCAL/PERSONAL USE ONLY. This scrapes Google Flights, which is against
// their Terms of Service like essentially all flight-data scraping. It must
// never be selected in any deployed environment — see FareProviderKind's
// wiring in internal/worker/runner.go, which only activates this when
// FARE_PROVIDER=googleflights is explicitly set, and deploy.yml never sets
// that. Do not add it there.
type GoogleFlightsScraper struct {
	pythonBin  string
	scriptPath string
	timeout    time.Duration
}

// NewGoogleFlightsScraper creates a GoogleFlightsScraper. pythonBin is
// typically the venv's python3 (see scripts/fare-scrape-poc/README.md);
// scriptPath is the path to query_json.py.
func NewGoogleFlightsScraper(pythonBin, scriptPath string) *GoogleFlightsScraper {
	return &GoogleFlightsScraper{
		pythonBin:  pythonBin,
		scriptPath: scriptPath,
		timeout:    30 * time.Second,
	}
}

type googleFlightsResult struct {
	PriceUSD       int    `json:"price_usd"`
	Airline        string `json:"airline"`
	Stops          int    `json:"stops"`
	DurationMin    int    `json:"duration_min"`
	DepartureLocal string `json:"departure_local"`
	FromAirport    string `json:"from_airport"`
	ToAirport      string `json:"to_airport"`
}

func (g *GoogleFlightsScraper) Search(ctx context.Context, req SearchRequest) ([]Fare, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, g.pythonBin, g.scriptPath, req.Origin, req.Destination, req.Date)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"google flights scraper (%s): %w — stderr: %s (has scripts/fare-scrape-poc been set up per its README?)",
			g.pythonBin, err, stderr.String(),
		)
	}

	var results []googleFlightsResult
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return nil, fmt.Errorf("parse query_json.py output: %w", err)
	}

	return mapGoogleFlightsResults(results, req), nil
}

func mapGoogleFlightsResults(results []googleFlightsResult, req SearchRequest) []Fare {
	out := make([]Fare, 0, len(results))
	for _, r := range results {
		dep, err := time.Parse("2006-01-02T15:04:05", r.DepartureLocal)
		if err != nil {
			continue
		}
		out = append(out, Fare{
			Origin:             req.Origin,
			Destination:        req.Destination,
			OriginAirport:      r.FromAirport,
			DestinationAirport: r.ToAirport,
			Airline:            r.Airline,
			DepartureAt:        dep,
			Price:              r.PriceUSD * 100, // dollars -> cents, matching the app's convention
			Currency:           "USD",
			Duration:           time.Duration(r.DurationMin) * time.Minute,
			Transfers:          r.Stops,
			Link:               googleFlightsBookingLink(req),
		})
	}
	return out
}

func googleFlightsBookingLink(req SearchRequest) string {
	return fmt.Sprintf(
		"https://www.google.com/travel/flights?q=Flights%%20to%%20%s%%20from%%20%s%%20on%%20%s",
		req.Destination, req.Origin, req.Date,
	)
}
