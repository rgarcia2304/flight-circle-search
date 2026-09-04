//go:build live
// +build live

package fareprovider

import (
	"context"
	"os"
	"testing"
)

func TestLiveSearch_NYCTOLON(t *testing.T) {
	token := os.Getenv("TRAVELPAYOUTS_TOKEN")
	if token == "" {
		t.Fatal("TRAVELPAYOUTS_TOKEN env var required")
	}

	client := NewTravelpayouts(token, "", nil)
	fares, err := client.Search(context.Background(), SearchRequest{
		Origin:      "NYC",
		Destination: "LON",
		Date:        "2026-10-15",
	})

	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(fares) == 0 {
		t.Fatal("expected at least one fare, got 0 (may need a date with recent search activity)")
	}

	f := fares[0]
	t.Logf("Cheapest fare: %s %s %s %s $%.2f %s departing %s",
		f.Origin, f.Destination, f.OriginAirport, f.DestinationAirport,
		float64(f.Price)/100, f.Currency, f.DepartureAt.Format("2006-01-02"))

	if f.Price <= 0 {
		t.Errorf("Price = %d, want > 0", f.Price)
	}
	if f.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", f.Currency)
	}
	if f.DepartureAt.IsZero() {
		t.Error("DepartureAt is zero")
	}
	if f.Link == "" {
		t.Error("Link is empty")
	}
	if f.OriginAirport == "" || f.DestinationAirport == "" {
		t.Error("Airport codes should be populated")
	}
}
