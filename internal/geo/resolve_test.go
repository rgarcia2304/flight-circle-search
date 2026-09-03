package geo

import (
	"math"
	"sort"
	"testing"
)

func northeastAirports() []Airport {
	return []Airport{
		{IATA: "JFK", Name: "John F. Kennedy International", Location: Point{Lat: 40.6413, Lng: -73.7781}},
		{IATA: "EWR", Name: "Newark Liberty International", Location: Point{Lat: 40.6895, Lng: -74.1745}},
		{IATA: "LGA", Name: "LaGuardia", Location: Point{Lat: 40.7769, Lng: -73.8740}},
		{IATA: "PHL", Name: "Philadelphia International", Location: Point{Lat: 39.8744, Lng: -75.2424}},
		{IATA: "BOS", Name: "Boston Logan International", Location: Point{Lat: 42.3656, Lng: -71.0096}},
		{IATA: "BWI", Name: "Baltimore/Washington International", Location: Point{Lat: 39.1754, Lng: -76.6684}},
		{IATA: "DCA", Name: "Ronald Reagan Washington National", Location: Point{Lat: 38.8521, Lng: -77.0377}},
		{IATA: "LHR", Name: "London Heathrow", Location: Point{Lat: 51.4700, Lng: -0.4543}},
		{IATA: "CDG", Name: "Paris Charles de Gaulle", Location: Point{Lat: 49.0097, Lng: 2.5479}},
		{IATA: "AMS", Name: "Amsterdam Schiphol", Location: Point{Lat: 52.3105, Lng: 4.7683}},
		{IATA: "FRA", Name: "Frankfurt", Location: Point{Lat: 50.0379, Lng: 8.5622}},
		{IATA: "MAD", Name: "Madrid Barajas", Location: Point{Lat: 40.4983, Lng: -3.5676}},
	}
}

func iatas(as []Airport) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.IATA
	}
	sort.Strings(out)
	return out
}

func TestResolveAirports_HappyPath_NYC_Center_500km(t *testing.T) {
	center := Point{Lat: 40.6413, Lng: -73.7781}
	airports := northeastAirports()

	got, err := ResolveAirports(center, 500, airports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"BOS", "BWI", "DCA", "EWR", "JFK", "LGA", "PHL"}
	if !equalStringSets(iatas(got), want) {
		t.Errorf("got %v, want %v", iatas(got), want)
	}
}

func TestResolveAirports_ZeroRadius(t *testing.T) {
	center := Point{Lat: 40.6413, Lng: -73.7781}
	airports := northeastAirports()

	got, err := ResolveAirports(center, 0, airports)
	if err == nil {
		if len(got) != 0 {
			t.Errorf("zero radius with no error: expected empty slice, got %v", iatas(got))
		}
		return
	}
	for _, a := range got {
		if a.IATA == "" {
			t.Errorf("expected nil/empty airports with error, got non-empty: %v", got)
		}
	}
}

func TestResolveAirports_NegativeRadius(t *testing.T) {
	center := Point{Lat: 40.6413, Lng: -73.7781}
	airports := northeastAirports()

	got, err := ResolveAirports(center, -100, airports)
	if err == nil {
		if len(got) != 0 {
			t.Errorf("negative radius with no error: expected empty slice, got %v", iatas(got))
		}
		return
	}
	for _, a := range got {
		if a.IATA == "" {
			t.Errorf("expected nil/empty airports with error, got non-empty: %v", got)
		}
	}
}

func TestResolveAirports_InvalidLatTooHigh(t *testing.T) {
	center := Point{Lat: 95.0, Lng: 0.0}
	airports := northeastAirports()

	_, err := ResolveAirports(center, 500, airports)
	if err == nil {
		t.Errorf("expected error for lat=95, got nil")
	}
}

func TestResolveAirports_InvalidLatTooLow(t *testing.T) {
	center := Point{Lat: -91.0, Lng: 0.0}
	airports := northeastAirports()

	_, err := ResolveAirports(center, 500, airports)
	if err == nil {
		t.Errorf("expected error for lat=-91, got nil")
	}
}

func TestResolveAirports_InvalidLngTooHigh(t *testing.T) {
	center := Point{Lat: 0.0, Lng: 181.0}
	airports := northeastAirports()

	_, err := ResolveAirports(center, 500, airports)
	if err == nil {
		t.Errorf("expected error for lng=181, got nil")
	}
}

func TestResolveAirports_InvalidLngTooLow(t *testing.T) {
	center := Point{Lat: 0.0, Lng: -181.0}
	airports := northeastAirports()

	_, err := ResolveAirports(center, 500, airports)
	if err == nil {
		t.Errorf("expected error for lng=-181, got nil")
	}
}

func TestResolveAirports_Antimeridian_IncludesFarSide(t *testing.T) {
	center := Point{Lat: 0.0, Lng: 179.0}
	airports := []Airport{
		{IATA: "NEAR_WEST", Location: Point{Lat: 0.0, Lng: 178.0}},
		{IATA: "NEAR_EAST", Location: Point{Lat: 0.0, Lng: -179.0}},
		{IATA: "FAR", Location: Point{Lat: 0.0, Lng: 0.0}},
	}

	got, err := ResolveAirports(center, 500, airports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"NEAR_EAST", "NEAR_WEST"}
	if !equalStringSets(iatas(got), want) {
		t.Errorf("antimeridian crossing: got %v, want %v", iatas(got), want)
	}
}

func TestResolveAirports_EmptyInput(t *testing.T) {
	center := Point{Lat: 40.6413, Lng: -73.7781}

	got, err := ResolveAirports(center, 500, []Airport{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Errorf("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", iatas(got))
	}
}

func TestResolveAirports_VeryLargeRadius(t *testing.T) {
	center := Point{Lat: 0.0, Lng: 0.0}
	airports := northeastAirports()

	got, err := ResolveAirports(center, math.MaxFloat64, airports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"AMS", "BOS", "BWI", "CDG", "DCA", "EWR", "FRA", "JFK", "LGA", "LHR", "MAD", "PHL"}
	if !equalStringSets(iatas(got), want) {
		t.Errorf("very large radius: got %v, want %v", iatas(got), want)
	}
}

func TestResolveAirports_DuplicatesDeduped(t *testing.T) {
	center := Point{Lat: 40.6413, Lng: -73.7781}
	jfk := Airport{IATA: "JFK", Name: "John F. Kennedy International", Location: Point{Lat: 40.6413, Lng: -73.7781}}
	ewr := Airport{IATA: "EWR", Name: "Newark Liberty International", Location: Point{Lat: 40.6895, Lng: -74.1745}}
	lga := Airport{IATA: "LGA", Name: "LaGuardia", Location: Point{Lat: 40.7769, Lng: -73.8740}}

	airports := []Airport{jfk, ewr, lga, jfk, ewr, jfk}

	got, err := ResolveAirports(center, 500, airports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"EWR", "JFK", "LGA"}
	if !equalStringSets(iatas(got), want) {
		t.Errorf("duplicates not deduped: got %v, want %v", iatas(got), want)
	}

	counts := map[string]int{}
	for _, a := range got {
		counts[a.IATA]++
	}
	for code, n := range counts {
		if n > 1 {
			t.Errorf("airport %s appears %d times in output", code, n)
		}
	}
}

// Test that invalid airports in the input list are skipped rather than causing the whole call to fail
func TestResolveAirports_SkipsInvalidAirports(t *testing.T) {
	center := Point{Lat: 40.6413, Lng: -73.7781}
	jfk := Airport{IATA: "JFK", Name: "John F. Kennedy International", Location: Point{Lat: 40.6413, Lng: -73.7781}}
	ewr := Airport{IATA: "EWR", Name: "Newark Liberty International", Location: Point{Lat: 40.6895, Lng: -74.1745}}
	invalidLat := Airport{IATA: "BAD", Name: "Invalid Airport", Location: Point{Lat: 95.0, Lng: 0.0}} // Invalid lat > 90

	airports := []Airport{jfk, ewr, invalidLat, jfk}

	got, err := ResolveAirports(center, 500, airports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"EWR", "JFK"}
	if !equalStringSets(iatas(got), want) {
		t.Errorf("skipping invalid airports: got %v, want %v", iatas(got), want)
	}
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int, len(a))
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
		if m[s] < 0 {
			return false
		}
	}
	return true
}

func TestHaversineDistanceKM_ExactValues(t *testing.T) {
	p1 := Point{Lat: 40.6413, Lng: -73.7781}
	p2 := Point{Lat: 41.6413, Lng: -73.7781}
	got := HaversineDistanceKM(p1, p2)
	const kmPerDegreeLat = 111.195
	tolerance := 1.0
	if math.Abs(got - kmPerDegreeLat) > tolerance {
		t.Errorf("1 degree latitude: got %.2f km, want ~%.2f +- %.1f km", got, kmPerDegreeLat, tolerance)
	}
}

func TestHaversineDistanceKM_ExactValues_Longitude(t *testing.T) {
	p1 := Point{Lat: 40.6413, Lng: -73.7781}
	p2 := Point{Lat: 40.6413, Lng: -72.7781}
	got := HaversineDistanceKM(p1, p2)
	const kmPerDegreeLngAt40 = 84.39
	tolerance := 1.0
	if math.Abs(got - kmPerDegreeLngAt40) > tolerance {
		t.Errorf("1 degree longitude at 40.6413N: got %.2f km, want ~%.2f +- %.1f km", got, kmPerDegreeLngAt40, tolerance)
	}
}

func TestHaversineDistanceKM_Antipodal(t *testing.T) {
	p1 := Point{Lat: 0.0, Lng: 0.0}
	p2 := Point{Lat: 0.0, Lng: 180.0}
	got := HaversineDistanceKM(p1, p2)
	const halfCircumferenceKM = 20015.087 // π * 6371.0
	tolerance := 1.0
	if math.Abs(got - halfCircumferenceKM) > tolerance {
		t.Errorf("antipodal equatorial points: got %.2f km, want ~%.2f +- %.1f km", got, halfCircumferenceKM, tolerance)
	}
}