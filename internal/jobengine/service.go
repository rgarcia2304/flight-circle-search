package jobengine

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rgarcia2304/flight-circle-search/internal/geo"
	"github.com/rgarcia2304/flight-circle-search/internal/routes"
)

// AirportSource yields the airport list and route graph.
type AirportSource interface {
	Airports(ctx context.Context) ([]geo.Airport, error)
	RouteGraph(ctx context.Context) (routes.RouteGraph, error)
}

// Enqueuer submits a batch of search job args to the queue.
type Enqueuer interface {
	EnqueueBatch(ctx context.Context, results []SearchJobResult) error
}

// Service orchestrates job creation, tuple generation, and enqueueing.
type Service struct {
	repo     Repository
	airports AirportSource
	enqueuer Enqueuer
	maxPairs int
	hubs     []string
}

// DefaultHubs is the set of major hubs used to find 1-hop paths when the route
// graph lacks a direct origin→destination edge. A pair (A, B) is reachable if
// there is a hub H such that A→H and H→B both exist in the route graph.
var DefaultHubs = []string{
	"LHR", // London Heathrow
	"FRA", // Frankfurt
	"AMS", // Amsterdam
	"CDG", // Paris CDG
	"IST", // Istanbul
	"DUB", // Dublin
	"MAD", // Madrid
	"MUC", // Munich
	"ZRH", // Zurich
	"BCN", // Barcelona
}

func NewService(repo Repository, airports AirportSource, enqueuer Enqueuer) *Service {
	return &Service{
		repo:     repo,
		airports: airports,
		enqueuer: enqueuer,
		maxPairs: MaxPairsPerJob,
		hubs:     DefaultHubs,
	}
}

// Submit creates a new search job, persists all per-pair rows, reads back the
// auto-generated result IDs, enqueues them to the queue, and marks the job running.
func (s *Service) Submit(ctx context.Context, req SearchRequest) (uuid.UUID, int, error) {
	pairs, err := s.GenerateTuples(ctx, req)
	if err != nil {
		return uuid.Nil, 0, err
	}
	if len(pairs) > s.maxPairs {
		return uuid.Nil, len(pairs), fmt.Errorf("%w: %d pairs (max %d)", ErrTooManyPairs, len(pairs), s.maxPairs)
	}

	results := make([]SearchJobResult, len(pairs))
	for i, p := range pairs {
		results[i] = SearchJobResult{
			Origin:        p.Origin,
			Destination:   p.Destination,
			DepartureDate: p.Date,
		}
	}

	job := SearchJob{
		Status:     StatusPending,
		TotalPairs: len(results),
		Request:    req,
	}

	id, err := s.repo.CreateJob(ctx, job, results)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("persist job: %w", err)
	}

	// Read back the inserted result rows to get their auto-generated IDs.
	inserted, err := s.repo.GetPendingResults(ctx, id)
	if err != nil {
		return id, len(results), fmt.Errorf("fetch result ids: %w", err)
	}
	if err := s.enqueuer.EnqueueBatch(ctx, inserted); err != nil {
		return id, len(results), fmt.Errorf("enqueue: %w", err)
	}

	if err := s.repo.MarkJobStatus(ctx, id, StatusRunning, nil); err != nil {
		return id, len(results), fmt.Errorf("mark running: %w", err)
	}

	return id, len(results), nil
}

// GeneratedPair is a single (origin, destination, date) tuple for a job.
type GeneratedPair struct {
	Origin      string
	Destination string
	Date        string
}

// GenerateTuples expands a request into the full combinatorial set of valid (origin, destination, date) tuples.
// Uses the existing route graph to filter to known routes.
func (s *Service) GenerateTuples(ctx context.Context, req SearchRequest) ([]GeneratedPair, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	airports, err := s.airports.Airports(ctx)
	if err != nil {
		return nil, fmt.Errorf("load airports: %w", err)
	}
	graph, err := s.airports.RouteGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("load routes: %w", err)
	}

	origins, err := geo.ResolveAirports(geo.Point{Lat: req.OriginLat, Lng: req.OriginLng}, req.OriginR, airports)
	if err != nil {
		return nil, fmt.Errorf("resolve origins: %w", err)
	}
	destinations, err := geo.ResolveAirports(geo.Point{Lat: req.DestLat, Lng: req.DestLng}, req.DestR, airports)
	if err != nil {
		return nil, fmt.Errorf("resolve destinations: %w", err)
	}

	originAirports := toRoutesAirports(origins)
	destAirports := toRoutesAirports(destinations)
	pairs := routes.FilterByRoutesWithHubs(
		routes.GenerateAllPairs(originAirports, destAirports),
		graph,
		s.hubs,
	)

	dates, err := expandDateRange(req.DepartFrom, req.DepartTo)
	if err != nil {
		return nil, fmt.Errorf("date range: %w", err)
	}

	out := make([]GeneratedPair, 0, len(pairs)*len(dates))
	for _, p := range pairs {
		for _, d := range dates {
			out = append(out, GeneratedPair{
				Origin:      p.Origin.IATA,
				Destination: p.Destination.IATA,
				Date:        d,
			})
		}
	}
	return out, nil
}

func toRoutesAirports(ga []geo.Airport) []routes.Airport {
	out := make([]routes.Airport, len(ga))
	for i, a := range ga {
		out[i] = routes.Airport{IATA: a.IATA, Name: a.Name, Location: a.Location}
	}
	return out
}

func expandDateRange(from, to string) ([]string, error) {
	t1, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	t2, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	if t2.Before(t1) {
		return nil, fmt.Errorf("to before from")
	}
	var out []string
	for d := t1; !d.After(t2); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out, nil
}
