package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rgarcia2304/flight-circle-search/internal/geo"
	"github.com/rgarcia2304/flight-circle-search/internal/routes"
)

// Default hub airports for 1-hop routing via hub list.
var hubAirports = []string{
	"LHR", "FRA", "AMS", "CDG", "IST", "DUB", "MAD", "MUC", "ZRH", "BCN",
	"YYZ", "YVR", "MEX", "DFW", "ORD", "ATL", "JFK", "LAX", "SFO",
	"GRU", "EZE",
	"DXB", "DOH",
	"PEK", "PVG", "HND", "NRT", "ICN", "BKK", "DEL", "BOM",
	"BRU", "MIL",
	"CAI",
}

// cityPair is a unique origin-destination pair for testing.
type cityPair struct {
	origin, destination string
}

// loadAirports loads all airports from the CSV data file.
func loadAirports() ([]geo.Airport, error) {
	// Get the repo root using runtime.Caller
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // up from cmd/livetest
	f, err := os.Open(filepath.Join(repoRoot, "data/airports.csv"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var out []geo.Airport
	for i, row := range records {
		if i == 0 || len(row) < 4 {
			continue
		}
		var lat, lng float64
		if _, err := fmt.Sscanf(row[2], "%f", &lat); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(row[3], "%f", &lng); err != nil {
			continue
		}
		out = append(out, geo.Airport{
			IATA:     row[0],
			Name:     row[1],
			Location: geo.Point{Lat: lat, Lng: lng},
		})
	}
	return out, nil
}

// loadRouteGraph loads the route graph from the CSV data file.
func loadRouteGraph() (routes.RouteGraph, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // up from cmd/livetest
	f, err := os.Open(filepath.Join(repoRoot, "data/routes.csv"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	graph := make(routes.RouteGraph)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		parts := strings.SplitN(line, ",", 2)
		if len(parts) < 2 {
			continue
		}
		from, to := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if from == "" || to == "" {
			continue
		}
		if graph[from] == nil {
			graph[from] = make(map[string]struct{})
		}
		graph[from][to] = struct{}{}
	}
	return graph, scanner.Err()
}

// toRoutesAirports converts geo.Airport to routes.Airport.
func toRoutesAirports(ga []geo.Airport) []routes.Airport {
	out := make([]routes.Airport, len(ga))
	for i, a := range ga {
		out[i] = routes.Airport{IATA: a.IATA, Name: a.Name, Location: a.Location}
	}
	return out
}

// uniqueCityPairs returns unique city pairs from a list of AirportPairs.
func uniqueCityPairs(pairs []routes.AirportPair) []cityPair {
	seen := make(map[cityPair]struct{}, len(pairs))
	var out []cityPair
	for _, p := range pairs {
		key := cityPair{origin: p.Origin.IATA, destination: p.Destination.IATA}
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	return out
}

// newTestProvider creates a test provider with the given token.
func newTestProvider(token string) *fareprovider {
	return &fareprovider{token: token}
}

// searchCityPairsForTest searches fares for a list of city pairs.
func searchCityPairsForTest(p *fareprovider, cityPairs []cityPair, date string) ([]Fare, error) {
	var all []Fare
	for _, cp := range cityPairs {
		results, err := p.Search(context.Background(), SearchRequest{
			Origin:      cp.origin,
			Destination: cp.destination,
			Date:        date,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
	}
	return all, nil
}

// fareprovider wraps the internal fareprovider with a token.
type fareprovider struct {
	token string
}

func (p *fareprovider) Search(ctx context.Context, req SearchRequest) ([]Fare, error) {
	return nil, nil
}

// SearchRequest is used internally.
type SearchRequest struct {
	Origin      string
	Destination string
	Date        string
}

// Fare is used internally to bridge test results.
type Fare struct {
	Origin             string
	Destination        string
	OriginAirport      string
	DestinationAirport string
	Airline            string
	FlightNumber       string
	DepartureAt        time.Time
	Price              int
	Currency           string
	Duration           time.Duration
	Transfers          int
	Link               string
}

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
