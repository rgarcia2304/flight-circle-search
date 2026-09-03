package geo

import (
	"math"
)

const earthRadiusKm = 6371.0

type Point struct {
	Lat float64
	Lng float64
}

type Airport struct {
	IATA     string
	Name     string
	Location Point
}

func HaversineDistanceKM(a, b Point) float64 {
	lat1, lon1 := a.Lat, a.Lng
	lat2, lon2 := b.Lat, b.Lng

	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0

	lat1r := lat1 * math.Pi / 180.0
	lat2r := lat2 * math.Pi / 180.0

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1r)*math.Cos(lat2r)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusKm * c
}

func isValidPoint(p Point) bool {
	if p.Lat < -90 || p.Lat > 90 {
		return false
	}
	if p.Lng < -180 || p.Lng > 180 {
		return false
	}
	return true
}

func ResolveAirports(center Point, radiusKm float64, airports []Airport) ([]Airport, error) {
	if radiusKm <= 0 {
		return []Airport{}, nil
	}

	if !isValidPoint(center) {
		return nil, errInvalidCoordinates
	}

	result := make([]Airport, 0, len(airports))
	seen := make(map[string]bool)

	for _, a := range airports {
		if !isValidPoint(a.Location) {
			continue
		}

		dist := HaversineDistanceKM(center, a.Location)
		if dist <= radiusKm {
			if !seen[a.IATA] {
				seen[a.IATA] = true
				result = append(result, a)
			}
		}
	}

	return result, nil
}

var errInvalidCoordinates = &errInvalidCoordinatesType{}

type errInvalidCoordinatesType struct{}

func (errInvalidCoordinatesType) Error() string {
	return "invalid coordinates"
}