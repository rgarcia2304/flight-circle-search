package routes

import (
	"sort"
	"testing"
)

func airport(iata string) Airport {
	return Airport{IATA: iata}
}

func emptyGraph() RouteGraph {
	return make(RouteGraph)
}

func graphWith(routes ...[2]string) RouteGraph {
	g := make(RouteGraph)
	for _, r := range routes {
		if g[r[0]] == nil {
			g[r[0]] = make(map[string]struct{})
		}
		g[r[0]][r[1]] = struct{}{}
	}
	return g
}

func iataPairs(pairs []AirportPair) []string {
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.Origin.IATA + "->" + p.Destination.IATA
	}
	sort.Strings(out)
	return out
}

func equalPairSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GenerateAllPairs tests

func TestGenerateAllPairs_Basic(t *testing.T) {
	origins := []Airport{airport("JFK"), airport("LHR")}
	destinations := []Airport{airport("CDG"), airport("FRA")}

	got := GenerateAllPairs(origins, destinations)
	want := []string{"JFK->CDG", "JFK->FRA", "LHR->CDG", "LHR->FRA"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("got %v, want %v", iataPairs(got), want)
	}
}

func TestGenerateAllPairs_EmptyOrigins(t *testing.T) {
	destinations := []Airport{airport("CDG")}
	got := GenerateAllPairs([]Airport{}, destinations)
	if len(got) != 0 {
		t.Errorf("empty origins: got %v, want empty", iataPairs(got))
	}
}

func TestGenerateAllPairs_EmptyDestinations(t *testing.T) {
	origins := []Airport{airport("JFK")}
	got := GenerateAllPairs(origins, []Airport{})
	if len(got) != 0 {
		t.Errorf("empty destinations: got %v, want empty", iataPairs(got))
	}
}

func TestGenerateAllPairs_SelfPairsExcluded(t *testing.T) {
	origins := []Airport{airport("JFK"), airport("LHR")}
	destinations := []Airport{airport("JFK"), airport("LHR")}

	got := GenerateAllPairs(origins, destinations)
	want := []string{"JFK->LHR", "LHR->JFK"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("self-pairs excluded: got %v, want %v", iataPairs(got), want)
	}
}

func TestGenerateAllPairs_SingleOriginSingleDestination(t *testing.T) {
	origins := []Airport{airport("JFK")}
	destinations := []Airport{airport("LHR")}

	got := GenerateAllPairs(origins, destinations)
	want := []string{"JFK->LHR"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("single pair: got %v, want %v", iataPairs(got), want)
	}
}

func TestGenerateAllPairs_OriginEqualsDestination(t *testing.T) {
	origins := []Airport{airport("JFK")}
	destinations := []Airport{airport("JFK")}

	got := GenerateAllPairs(origins, destinations)
	if len(got) != 0 {
		t.Errorf("same airport: got %v, want empty", iataPairs(got))
	}
}

// FilterByRoutes tests

func TestFilterByRoutes_EmptyPairs(t *testing.T) {
	routes := graphWith([2]string{"JFK", "LHR"})
	got := FilterByRoutes([]AirportPair{}, routes)
	if len(got) != 0 {
		t.Errorf("empty pairs: got %v, want empty", iataPairs(got))
	}
}

func TestFilterByRoutes_EmptyRouteGraph(t *testing.T) {
	pairs := []AirportPair{
		{Origin: airport("JFK"), Destination: airport("LHR")},
	}
	got := FilterByRoutes(pairs, emptyGraph())
	if len(got) != 0 {
		t.Errorf("empty graph: got %v, want empty", iataPairs(got))
	}
}

func TestFilterByRoutes_ForwardDirection(t *testing.T) {
	routes := graphWith([2]string{"JFK", "LHR"})
	pairs := []AirportPair{
		{Origin: airport("JFK"), Destination: airport("LHR")},
	}

	got := FilterByRoutes(pairs, routes)
	if len(got) != 1 {
		t.Errorf("forward direction: got %v, want 1 pair", iataPairs(got))
	}
}

func TestFilterByRoutes_ReverseDirection(t *testing.T) {
	routes := graphWith([2]string{"LHR", "JFK"})
	pairs := []AirportPair{
		{Origin: airport("JFK"), Destination: airport("LHR")},
	}

	got := FilterByRoutes(pairs, routes)
	if len(got) != 1 {
		t.Errorf("reverse direction: got %v, want 1 pair", iataPairs(got))
	}
}

func TestFilterByRoutes_NoRoute(t *testing.T) {
	routes := graphWith([2]string{"JFK", "CDG"})
	pairs := []AirportPair{
		{Origin: airport("JFK"), Destination: airport("LHR")},
	}

	got := FilterByRoutes(pairs, routes)
	if len(got) != 0 {
		t.Errorf("no route: got %v, want empty", iataPairs(got))
	}
}

func TestFilterByRoutes_MultiplePairs(t *testing.T) {
	routes := graphWith(
		[2]string{"JFK", "CDG"},
		[2]string{"LHR", "FRA"},
	)
	pairs := []AirportPair{
		{Origin: airport("JFK"), Destination: airport("CDG")},
		{Origin: airport("JFK"), Destination: airport("LHR")},
		{Origin: airport("LHR"), Destination: airport("FRA")},
		{Origin: airport("LHR"), Destination: airport("AMS")},
	}

	got := FilterByRoutes(pairs, routes)
	want := []string{"JFK->CDG", "LHR->FRA"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("multiple pairs: got %v, want %v", iataPairs(got), want)
	}
}

// FilterExistingRoutes integration tests

func TestFilterExistingRoutes_EmptyOrigins(t *testing.T) {
	origins := []Airport{}
	destinations := []Airport{airport("JFK"), airport("LHR")}
	routes := graphWith([2]string{"JFK", "LHR"})

	got := FilterExistingRoutes(origins, destinations, routes)
	if len(got) != 0 {
		t.Errorf("empty origins: got %v, want empty", iataPairs(got))
	}
}

func TestFilterExistingRoutes_EmptyDestinations(t *testing.T) {
	origins := []Airport{airport("JFK"), airport("LHR")}
	destinations := []Airport{}
	routes := graphWith([2]string{"JFK", "LHR"})

	got := FilterExistingRoutes(origins, destinations, routes)
	if len(got) != 0 {
		t.Errorf("empty destinations: got %v, want empty", iataPairs(got))
	}
}

func TestFilterExistingRoutes_EmptyRouteGraph(t *testing.T) {
	origins := []Airport{airport("JFK"), airport("LHR")}
	destinations := []Airport{airport("CDG"), airport("FRA")}
	routes := emptyGraph()

	got := FilterExistingRoutes(origins, destinations, routes)
	if len(got) != 0 {
		t.Errorf("empty route graph: got %v, want empty", iataPairs(got))
	}
}

func TestFilterExistingRoutes_ReverseDirectionIncluded(t *testing.T) {
	origins := []Airport{airport("JFK")}
	destinations := []Airport{airport("LHR")}
	routes := graphWith([2]string{"LHR", "JFK"})

	got := FilterExistingRoutes(origins, destinations, routes)
	if len(got) != 1 {
		t.Errorf("reverse direction: got %v, want 1 pair", iataPairs(got))
		return
	}
	if got[0].Origin.IATA != "JFK" || got[0].Destination.IATA != "LHR" {
		t.Errorf("reverse direction: got %v, want JFK->LHR", iataPairs(got))
	}
}

func TestFilterExistingRoutes_SelfLoopExcluded(t *testing.T) {
	origins := []Airport{airport("JFK")}
	destinations := []Airport{airport("JFK")}
	routes := graphWith([2]string{"JFK", "JFK"})

	got := FilterExistingRoutes(origins, destinations, routes)
	if len(got) != 0 {
		t.Errorf("self-loop: got %v, want empty", iataPairs(got))
	}
}

func TestFilterExistingRoutes_MultiplePairs(t *testing.T) {
	origins := []Airport{airport("JFK"), airport("LHR")}
	destinations := []Airport{airport("CDG"), airport("FRA")}
	routes := graphWith(
		[2]string{"JFK", "CDG"},
		[2]string{"LHR", "FRA"},
	)

	got := FilterExistingRoutes(origins, destinations, routes)
	want := []string{"JFK->CDG", "LHR->FRA"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("multiple pairs: got %v, want %v", iataPairs(got), want)
	}
}

func TestFilterExistingRoutes_LargeGraphPerformance(t *testing.T) {
	origins := []Airport{airport("JFK")}
	destinations := []Airport{airport("LHR")}

	routes := make(RouteGraph)
	for i := 0; i < 5000; i++ {
		from := airportCode("A", i)
		to := airportCode("B", i)
		if routes[from] == nil {
			routes[from] = make(map[string]struct{})
		}
		routes[from][to] = struct{}{}
	}
	routes["JFK"] = make(map[string]struct{})
	routes["JFK"]["LHR"] = struct{}{}

	got := FilterExistingRoutes(origins, destinations, routes)
	if len(got) != 1 {
		t.Errorf("large graph: got %v, want 1 pair", iataPairs(got))
		return
	}
	if got[0].Origin.IATA != "JFK" || got[0].Destination.IATA != "LHR" {
		t.Errorf("large graph: got %v, want JFK->LHR", iataPairs(got))
	}
}

func airportCode(prefix string, i int) string {
	return prefix + string(rune('A'+i/26)) + string(rune('A'+i%26))
}

// ---------------------------------------------------------------------------
// FilterByRoutesWithHubs tests
// ---------------------------------------------------------------------------

func TestFilterByRoutesWithHubs_DirectRoute_Preserved(t *testing.T) {
	g := graphWith([2]string{"JFK", "LHR"})
	pairs := []AirportPair{{Origin: airport("JFK"), Destination: airport("LHR")}}
	got := FilterByRoutesWithHubs(pairs, g, []string{"FRA", "AMS"})
	want := []string{"JFK->LHR"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("direct route: got %v, want %v", iataPairs(got), want)
	}
}

func TestFilterByRoutesWithHubs_OneHopViaHub_Passes(t *testing.T) {
	g := graphWith([2]string{"BOS", "FRA"}, [2]string{"FRA", "TXL"})
	pairs := []AirportPair{{Origin: airport("BOS"), Destination: airport("TXL")}}
	got := FilterByRoutesWithHubs(pairs, g, []string{"FRA", "AMS"})
	want := []string{"BOS->TXL"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("1-hop via FRA: got %v, want %v", iataPairs(got), want)
	}
}

func TestFilterByRoutesWithHubs_NoPath_Excluded(t *testing.T) {
	g := graphWith([2]string{"BOS", "CDG"})
	pairs := []AirportPair{{Origin: airport("BOS"), Destination: airport("TXL")}}
	got := FilterByRoutesWithHubs(pairs, g, []string{"FRA", "AMS"})
	if len(got) != 0 {
		t.Errorf("no path: got %v, want empty", iataPairs(got))
	}
}

func TestFilterByRoutesWithHubs_EmptyHubsList_EquivalentToDirect(t *testing.T) {
	g := graphWith([2]string{"JFK", "LHR"}, [2]string{"JFK", "FRA"}, [2]string{"FRA", "LHR"})
	pairs := []AirportPair{{Origin: airport("JFK"), Destination: airport("LHR")}}

	withHubs := FilterByRoutesWithHubs(pairs, g, nil)
	direct := FilterByRoutes(pairs, g)

	if !equalPairSets(iataPairs(withHubs), iataPairs(direct)) {
		t.Errorf("empty hubs: got %v, want %v", iataPairs(withHubs), iataPairs(direct))
	}
}

func TestFilterByRoutesWithHubs_ReverseDirection_AlsoWorks(t *testing.T) {
	g := graphWith([2]string{"FRA", "BOS"}, [2]string{"TXL", "FRA"})
	pairs := []AirportPair{{Origin: airport("BOS"), Destination: airport("TXL")}}
	got := FilterByRoutesWithHubs(pairs, g, []string{"FRA"})
	want := []string{"BOS->TXL"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("reverse direction: got %v, want %v", iataPairs(got), want)
	}
}

func TestFilterByRoutesWithHubs_MultipleHubs_TriesAll(t *testing.T) {
	g := graphWith([2]string{"BOS", "AMS"}, [2]string{"AMS", "TXL"})
	pairs := []AirportPair{{Origin: airport("BOS"), Destination: airport("TXL")}}
	got := FilterByRoutesWithHubs(pairs, g, []string{"FRA", "AMS", "CDG"})
	want := []string{"BOS->TXL"}
	if !equalPairSets(iataPairs(got), want) {
		t.Errorf("multiple hubs: got %v, want %v", iataPairs(got), want)
	}
}