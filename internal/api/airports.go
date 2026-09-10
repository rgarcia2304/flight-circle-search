package api

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rgarcia2304/flight-circle-search/internal/geo"
	"github.com/rgarcia2304/flight-circle-search/internal/routes"
)

type csvAirportSource struct {
	baseDir string
}

func newCSVAirportSource() *csvAirportSource {
	if _, err := os.Stat("data/airports.csv"); err == nil {
		return &csvAirportSource{}
	}
	if _, err := os.Stat("../data/airports.csv"); err == nil {
		return &csvAirportSource{baseDir: ".."}
	}
	// Fallback: derive repo root from location of this source file.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // up from internal/api
	return &csvAirportSource{baseDir: repoRoot}
}

func (s *csvAirportSource) Airports(_ context.Context) ([]geo.Airport, error) {
	f, err := os.Open(filepath.Join(s.baseDir, "data/airports.csv"))
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

func (s *csvAirportSource) RouteGraph(_ context.Context) (routes.RouteGraph, error) {
	f, err := os.Open(filepath.Join(s.baseDir, "data/routes.csv"))
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
