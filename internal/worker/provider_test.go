package worker

import (
	"testing"

	"github.com/rgarcia2304/flight-circle-search/internal/config"
	"github.com/rgarcia2304/flight-circle-search/internal/fareprovider"
)

func TestNewFareProvider_DefaultsToTravelpayouts(t *testing.T) {
	for _, kind := range []string{"", "travelpayouts"} {
		fp, err := newFareProvider(config.Config{FareProviderKind: kind, TravelpayoutsAPIKey: "token"}, nil)
		if err != nil {
			t.Fatalf("FareProviderKind=%q: unexpected error: %v", kind, err)
		}
		if _, ok := fp.(*fareprovider.Travelpayouts); !ok {
			t.Errorf("FareProviderKind=%q: got %T, want *fareprovider.Travelpayouts", kind, fp)
		}
	}
}

func TestNewFareProvider_GoogleFlightsOptIn(t *testing.T) {
	fp, err := newFareProvider(config.Config{
		FareProviderKind:        "googleflights",
		GoogleFlightsPythonBin:  "python3",
		GoogleFlightsScriptPath: "query_json.py",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := fp.(*fareprovider.GoogleFlightsScraper); !ok {
		t.Errorf("got %T, want *fareprovider.GoogleFlightsScraper", fp)
	}
}

func TestNewFareProvider_UnknownKindErrors(t *testing.T) {
	_, err := newFareProvider(config.Config{FareProviderKind: "bogus"}, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown FareProviderKind, got nil")
	}
}
