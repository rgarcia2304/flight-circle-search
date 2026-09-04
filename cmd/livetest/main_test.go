//go:build live
// +build live

package main

import (
	"os"
	"sort"
	"testing"

	"github.com/rgarcia2304/flight-circle-search/internal/geo"
	"github.com/rgarcia2304/flight-circle-search/internal/routes"
)

func TestLiveSearch_FullPipeline_Corridors(t *testing.T) {
	token := os.Getenv("TRAVELPAYOUTS_TOKEN")
	if token == "" {
		t.Fatal("TRAVELPAYOUTS_TOKEN env var required")
	}

	airports, err := loadAirports()
	if err != nil {
		t.Fatalf("load airports: %v", err)
	}

	graph, err := loadRouteGraph()
	if err != nil {
		t.Fatalf("load routes: %v", err)
	}

	type corridorSpec struct {
		name       string
		orig       geo.Point
		origR      float64
		dest       geo.Point
		destR      float64
		connecting bool
	}

	berlinOrig := geo.Point{Lat: 40.6413, Lng: -73.7781}
	berlinDest := geo.Point{Lat: 52.3667, Lng: 13.5033}
	londonOrig := geo.Point{Lat: 40.6413, Lng: -73.7781}
	londonDest := geo.Point{Lat: 51.4700, Lng: -0.4543}

	corridors := []corridorSpec{
		{
			name:  "NYC → London (200mi both, direct only)",
			orig:  londonOrig,
			origR: 322,
			dest:  londonDest,
			destR: 322,
		},
		{
			name:  "NYC → Berlin (200mi / 400mi, direct only)",
			orig:  berlinOrig,
			origR: 322,
			dest:  berlinDest,
			destR: 644,
		},
		{
			name:       "NYC → Berlin (200mi / 400mi, with 1-hop hubs)",
			orig:       berlinOrig,
			origR:      322,
			dest:       berlinDest,
			destR:      644,
			connecting: true,
		},
	}

	directBerlinPairs := computeDirectCityPairs(t, berlinOrig, 322, berlinDest, 644, airports, graph)
	t.Logf("Direct-only NYC→Berlin city pairs (baseline): %v", directBerlinPairs)

	for _, c := range corridors {
		t.Run(c.name, func(t *testing.T) {
			origins, err := geo.ResolveAirports(c.orig, c.origR, airports)
			if err != nil {
				t.Fatalf("resolve origin: %v", err)
			}
			destinations, err := geo.ResolveAirports(c.dest, c.destR, airports)
			if err != nil {
				t.Fatalf("resolve dest: %v", err)
			}

			var pairs []routes.AirportPair
			if c.connecting {
				allPairs := routes.GenerateAllPairs(
					toRoutesAirports(origins),
					toRoutesAirports(destinations),
				)
				pairs = routes.FilterByRoutesWithHubs(allPairs, graph, hubAirports)
			} else {
				pairs = routes.FilterExistingRoutes(
					toRoutesAirports(origins),
					toRoutesAirports(destinations),
					graph,
				)
			}

			cityPairs := uniqueCityPairs(pairs)
			t.Logf("Resolved %d origin airports, %d dest airports, %d city pairs: %v",
				len(origins), len(destinations), len(cityPairs), cityPairs)

			if len(cityPairs) < 1 {
				t.Errorf("expected at least 1 city pair, got %d", len(cityPairs))
			}

			provider := newTestProvider(token)
			results, err := searchCityPairsForTest(provider, cityPairs, "2026-10-15")
			if err != nil {
				t.Fatalf("search: %v", err)
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].Price < results[j].Price
			})

			t.Logf("Found %d fares", len(results))
			for i, f := range results {
				if i >= 10 {
					break
				}
				t.Logf("  %2d. $%7.2f  %s→%s (%s→%s)  %s %-8s  %s",
					i+1, float64(f.Price)/100,
					f.Origin, f.Destination,
					f.OriginAirport, f.DestinationAirport,
					f.Airline, f.FlightNumber,
					f.DepartureAt.Format("2006-01-02"))
			}

			if len(results) == 0 {
				t.Log("No fares found — this may be expected for future dates")
			} else {
				if results[0].Price <= 0 {
					t.Errorf("cheapest price = %d, want > 0", results[0].Price)
				}
				if results[0].Currency != "USD" {
					t.Errorf("currency = %q, want USD", results[0].Currency)
				}
				if results[0].DepartureAt.IsZero() {
					t.Error("cheapest departure date is zero")
				}
			}

			if c.connecting {
				baseline := make(map[string]bool, len(directBerlinPairs))
				for _, p := range directBerlinPairs {
					baseline[p.origin+"→"+p.destination] = true
				}
				newCities := 0
				newCitySet := make(map[string]bool)
				for _, f := range results {
					pair := f.Origin + "→" + f.Destination
					if !baseline[pair] {
						newCities++
						newCitySet[pair] = true
					}
				}
				if newCities == 0 {
					t.Errorf("connecting run should surface fares to cities not in direct-only run; got 0 new (baseline: %v)", baseline)
				} else {
					t.Logf("Connecting run surfaced %d fare(s) to new cities not in direct-only run: %v", newCities, newCitySet)
				}
			}
		})
	}
}

func computeDirectCityPairs(t *testing.T, orig geo.Point, origR float64, dest geo.Point, destR float64, airports []geo.Airport, graph routes.RouteGraph) []cityPair {
	t.Helper()
	origins, err := geo.ResolveAirports(orig, origR, airports)
	if err != nil {
		t.Fatalf("resolve origin: %v", err)
	}
	destinations, err := geo.ResolveAirports(dest, destR, airports)
	if err != nil {
		t.Fatalf("resolve dest: %v", err)
	}
	pairs := routes.FilterExistingRoutes(
		toRoutesAirports(origins),
		toRoutesAirports(destinations),
		graph,
	)
	return uniqueCityPairs(pairs)
}
