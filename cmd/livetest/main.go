package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/rgarcia2304/flight-circle-search/internal/fareprovider"
	"github.com/rgarcia2304/flight-circle-search/internal/geo"
	"github.com/rgarcia2304/flight-circle-search/internal/routes"
)

var (
	date        = flag.String("date", "", "departure date (YYYY-MM-DD)")
	originLat   = flag.Float64("origin-lat", 40.6413, "origin circle center latitude")
	originLng   = flag.Float64("origin-lng", -73.7781, "origin circle center longitude")
	originR     = flag.Float64("origin-r", 322, "origin circle radius in km (200 miles)")
	destLat     = flag.Float64("dest-lat", 52.3667, "destination circle center latitude (default: Berlin)")
	destLng     = flag.Float64("dest-lng", 13.5033, "destination circle center longitude (default: Berlin)")
	destR       = flag.Float64("dest-r", 644, "destination circle radius in km (default: 644km = 400mi)")
	connecting  = flag.Bool("connecting", false, "include 1-hop connecting routes via major hub airports")
	token       = flag.String("token", os.Getenv("TRAVELPAYOUTS_TOKEN"), "Travelpayouts API token")
)

var hubAirports = []string{
	"FRA", "AMS", "CDG", "LHR", "MAD",
	"MUC", "FCO", "BCN", "ZRH", "DUS", "BRU",
}

var airportToCity = map[string]string{
	// East Coast US
	"JFK": "NYC", "LGA": "NYC", "EWR": "NYC",
	"BOS": "BOS",
	"PHL": "PHL", "PNE": "PHL",
	"DCA": "WAS", "BWI": "WAS", "IAD": "WAS",
	"ALB": "ALB",
	"PVD": "PVD",
	"MDT": "MDT", "HAR": "MDT",
	// London
	"LHR": "LON", "LGW": "LON", "STN": "LON", "LTN": "LON",
	// Paris
	"CDG": "PAR", "ORY": "PAR", "BVA": "PAR",
	// Amsterdam
	"AMS": "AMS",
	// Frankfurt
	"FRA": "FRA",
	// Madrid
	"MAD": "MAD",
	// Berlin (center hub for 400mi radius)
	"TXL": "BER", "SXF": "BER", "THF": "BER",
	// Berlin area: Prague
	"PRG": "PRG",
	// Berlin area: Leipzig
	"LEJ": "LEJ",
	// Berlin area: Dresden
	"DRS": "DRS",
	// Berlin area: Hamburg
	"HAM": "HAM",
	// Berlin area: Hanover
	"HAJ": "HAJ",
	// Berlin area: Copenhagen
	"CPH": "CPH",
	// Berlin area: Wroclaw
	"WRO": "WRO",
	// Berlin area: Poznan
	"POZ": "POZ",
	// Berlin area: Bremen
	"BRE": "BRE",
	// Berlin area: Nuremberg
	"NUE": "NUE",
}

func main() {
	flag.Parse()

	if *date == "" || *token == "" {
		fmt.Println("Usage: livetest -date=YYYY-MM-DD [-origin-lat=X -origin-lng=Y -origin-r=KM -dest-lat=X -dest-lng=Y -dest-r=KM]")
		fmt.Println("Requires: TRAVELPAYOUTS_TOKEN env var or -token flag")
		os.Exit(1)
	}

	if err := run(*date); err != nil {
		log.Fatal(err)
	}
}

func run(date string) error {
	results, err := search(date)
	if err != nil {
		return err
	}

	fmt.Printf("\n=== Flight Search: %s ===\n", date)
	fmt.Printf("Origin circle: (%.4f, %.4f) ±%.0fkm\n",
		*originLat, *originLng, *originR)
	fmt.Printf("Dest circle:   (%.4f, %.4f) ±%.0fkm\n",
		*destLat, *destLng, *destR)
	fmt.Printf("Fares found: %d\n\n", len(results))

	if len(results) == 0 {
		fmt.Println("No fares found for the given corridor and date.")
		return nil
	}

	fmt.Printf("%-3s %-7s %-12s %-6s %-8s %-12s %s\n",
		"#", "PRICE", "ROUTE", "AIRLINE", "FLIGHT", "DEP_DATE", "DURATION")
	fmt.Println(strings.Repeat("-", 75))
	for i, f := range results {
		if i >= 20 {
			fmt.Printf("... and %d more results\n", len(results)-i)
			break
		}
		fmt.Printf("%-3d $%-6.2f %s→%s (%s→%s) %-6s %-8s %s %d min\n",
			i+1, float64(f.Price)/100,
			f.Origin, f.Destination,
			f.OriginAirport, f.DestinationAirport,
			f.Airline, f.FlightNumber,
			f.DepartureAt.Format("2006-01-02"),
			int(f.Duration.Minutes()))
	}
	return nil
}

func search(date string) ([]fareprovider.Fare, error) {
	airports, err := loadAirports()
	if err != nil {
		return nil, fmt.Errorf("load airports: %w", err)
	}

	graph, err := loadRouteGraph()
	if err != nil {
		return nil, fmt.Errorf("load routes: %w", err)
	}

	originCircle := geo.Point{Lat: *originLat, Lng: *originLng}
	destCircle := geo.Point{Lat: *destLat, Lng: *destLng}

	origins, err := geo.ResolveAirports(originCircle, *originR, airports)
	if err != nil {
		return nil, fmt.Errorf("resolve origin airports: %w", err)
	}

	destinations, err := geo.ResolveAirports(destCircle, *destR, airports)
	if err != nil {
		return nil, fmt.Errorf("resolve destination airports: %w", err)
	}

	var routeAirportPairs []routes.AirportPair
	if *connecting {
		allPairs := routes.GenerateAllPairs(
			toRoutesAirports(origins),
			toRoutesAirports(destinations),
		)
		routeAirportPairs = routes.FilterByRoutesWithHubs(allPairs, graph, hubAirports)
	} else {
		routeAirportPairs = routes.FilterExistingRoutes(
			toRoutesAirports(origins),
			toRoutesAirports(destinations),
			graph,
		)
	}

	cityPairs := uniqueCityPairs(routeAirportPairs)

	provider := fareprovider.NewTravelpayouts(*token, "", nil)

	results, err := searchCityPairs(context.Background(), provider, cityPairs, date)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Price < results[j].Price
	})

	return results, nil
}

type cityPair struct{ origin, destination string }

func uniqueCityPairs(pairs []routes.AirportPair) []cityPair {
	seen := make(map[cityPair]struct{})
	var result []cityPair
	for _, p := range pairs {
		cp := cityPair{
			origin:      airportToCity[p.Origin.IATA],
			destination: airportToCity[p.Destination.IATA],
		}
		if cp.origin == "" || cp.destination == "" {
			continue
		}
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		result = append(result, cp)
	}
	return result
}

func searchCityPairs(ctx context.Context, provider *fareprovider.Travelpayouts, pairs []cityPair, date string) ([]fareprovider.Fare, error) {
	var results []fareprovider.Fare
	for _, cp := range pairs {
		fares, err := provider.Search(ctx, fareprovider.SearchRequest{
			Origin:      cp.origin,
			Destination: cp.destination,
			Date:        date,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ! %s→%s: %v\n", cp.origin, cp.destination, err)
			continue
		}
		results = append(results, fares...)
		if len(pairs) > 5 {
			runtime.Gosched()
		}
	}
	return results, nil
}

func toRoutesAirports(ga []geo.Airport) []routes.Airport {
	out := make([]routes.Airport, len(ga))
	for i, a := range ga {
		out[i] = routes.Airport{IATA: a.IATA, Name: a.Name, Location: a.Location}
	}
	return out
}

func dataPath(name string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return name
	}
	if strings.HasSuffix(cwd, "/cmd/livetest") {
		return "../../" + name
	}
	return name
}

func loadAirports() ([]geo.Airport, error) {
	f, err := os.Open(dataPath("data/airports.csv"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}

	var airports []geo.Airport
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
		airports = append(airports, geo.Airport{
			IATA: row[0], Name: row[1],
			Location: geo.Point{Lat: lat, Lng: lng},
		})
	}
	return airports, nil
}

func loadRouteGraph() (routes.RouteGraph, error) {
	f, err := os.Open(dataPath("data/routes.csv"))
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
