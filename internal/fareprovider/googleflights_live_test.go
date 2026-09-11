//go:build live
// +build live

package fareprovider

import (
	"context"
	"os"
	"testing"
)

// TestLiveGoogleFlightsScraper exercises the real subprocess + real network
// path: run `scripts/fare-scrape-poc`'s venv setup first (see its README),
// then:
//
//	go test -tags=live ./internal/fareprovider/... -run TestLiveGoogleFlightsScraper -v
func TestLiveGoogleFlightsScraper(t *testing.T) {
	pythonBin := "../../scripts/fare-scrape-poc/venv/bin/python3"
	scriptPath := "../../scripts/fare-scrape-poc/query_json.py"
	if _, err := os.Stat(pythonBin); err != nil {
		t.Skipf("venv not set up at %s — see scripts/fare-scrape-poc/README.md", pythonBin)
	}

	g := NewGoogleFlightsScraper(pythonBin, scriptPath)
	fares, err := g.Search(context.Background(), SearchRequest{
		Origin:      "JFK",
		Destination: "LHR",
		Date:        "2026-10-15",
	})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(fares) == 0 {
		t.Fatal("expected at least one fare, got 0")
	}

	f := fares[0]
	t.Logf("Cheapest: %s %s %s $%.2f %s departing %s (via %s, %d stops)",
		f.Origin, f.Destination, f.Airline, float64(f.Price)/100, f.Currency,
		f.DepartureAt.Format("2006-01-02 15:04"), f.OriginAirport, f.Transfers)

	if f.Price <= 0 {
		t.Errorf("Price = %d, want > 0", f.Price)
	}
	if f.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", f.Currency)
	}
	if f.DepartureAt.IsZero() {
		t.Error("DepartureAt is zero")
	}
	if f.OriginAirport == "" || f.DestinationAirport == "" {
		t.Error("airport codes should be populated")
	}
}
