package fareprovider

import (
	"testing"
	"time"
)

func TestMapGoogleFlightsResults(t *testing.T) {
	req := SearchRequest{Origin: "JFK", Destination: "LHR", Date: "2026-10-15"}
	results := []googleFlightsResult{
		{
			PriceUSD:       295,
			Airline:        "British Airways",
			Stops:          0,
			DurationMin:    415,
			DepartureLocal: "2026-10-15T10:30:00",
			FromAirport:    "JFK",
			ToAirport:      "LHR",
		},
		{
			PriceUSD:       308,
			Airline:        "Aer Lingus",
			Stops:          1,
			DurationMin:    490,
			DepartureLocal: "2026-10-15T16:55:00",
			FromAirport:    "JFK",
			ToAirport:      "LHR",
		},
	}

	fares := mapGoogleFlightsResults(results, req)
	if len(fares) != 2 {
		t.Fatalf("got %d fares, want 2", len(fares))
	}

	f := fares[0]
	if f.Price != 29500 {
		t.Errorf("Price = %d, want 29500 (cents, not dollars)", f.Price)
	}
	if f.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", f.Currency)
	}
	if f.Origin != "JFK" || f.Destination != "LHR" {
		t.Errorf("Origin/Destination = %s/%s, want JFK/LHR", f.Origin, f.Destination)
	}
	if f.OriginAirport != "JFK" || f.DestinationAirport != "LHR" {
		t.Errorf("OriginAirport/DestinationAirport = %s/%s, want JFK/LHR", f.OriginAirport, f.DestinationAirport)
	}
	if f.Transfers != 0 {
		t.Errorf("Transfers = %d, want 0", f.Transfers)
	}
	if f.Duration != 415*time.Minute {
		t.Errorf("Duration = %v, want 415m", f.Duration)
	}
	wantDep := time.Date(2026, 10, 15, 10, 30, 0, 0, time.UTC)
	if !f.DepartureAt.Equal(wantDep) {
		t.Errorf("DepartureAt = %v, want %v", f.DepartureAt, wantDep)
	}
	if f.Link == "" {
		t.Error("expected a non-empty booking link")
	}
}

func TestMapGoogleFlightsResults_SkipsUnparseableDeparture(t *testing.T) {
	req := SearchRequest{Origin: "JFK", Destination: "LHR", Date: "2026-10-15"}
	results := []googleFlightsResult{
		{PriceUSD: 295, DepartureLocal: "not-a-timestamp"},
	}
	fares := mapGoogleFlightsResults(results, req)
	if len(fares) != 0 {
		t.Fatalf("got %d fares, want 0 (unparseable departure should be skipped)", len(fares))
	}
}
