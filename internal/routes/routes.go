package routes

import "github.com/rgarcia2304/flight-circle-search/internal/geo"

type Airport struct {
	IATA     string
	Name     string
	Location geo.Point
}

type RouteGraph map[string]map[string]struct{}

func (g RouteGraph) Has(from, to string) bool {
	dests, ok := g[from]
	if !ok {
		return false
	}
	_, exists := dests[to]
	return exists
}

type AirportPair struct {
	Origin      Airport
	Destination Airport
}

// GenerateAllPairs returns every origin × destination combination, excluding
// self-pairs (origin.IATA == destination.IATA). It does not consult the route
// graph.
func GenerateAllPairs(origins, destinations []Airport) []AirportPair {
	result := make([]AirportPair, 0, len(origins)*len(destinations))
	for _, o := range origins {
		for _, d := range destinations {
			if o.IATA == d.IATA {
				continue
			}
			result = append(result, AirportPair{Origin: o, Destination: d})
		}
	}
	return result
}

// FilterByRoutes returns the subset of pairs for which a route exists in
// either direction in the route graph. Pairs referencing IATA codes absent
// from the graph are excluded (not treated as an error).
func FilterByRoutes(pairs []AirportPair, graph RouteGraph) []AirportPair {
	result := make([]AirportPair, 0, len(pairs))
	for _, p := range pairs {
		if graph.Has(p.Origin.IATA, p.Destination.IATA) ||
			graph.Has(p.Destination.IATA, p.Origin.IATA) {
			result = append(result, p)
		}
	}
	return result
}

// FilterExistingRoutes is the convenience composition: generate all pairs
// from the origin/destination slices, then keep only the ones backed by a
// route in the graph.
func FilterExistingRoutes(origins, destinations []Airport, routes RouteGraph) []AirportPair {
	return FilterByRoutes(GenerateAllPairs(origins, destinations), routes)
}