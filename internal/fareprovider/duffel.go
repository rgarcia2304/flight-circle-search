package fareprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"
)

const (
	duffelDefaultBaseURL = "https://api.duffel.com"
	duffelDefaultTimeout = 30 * time.Second
	duffelVersionHeader  = "v2"
	duffelPollInterval   = 500 * time.Millisecond
	duffelMaxPolls       = 60 // 30s total
)

// Duffel is a FareProvider backed by Duffel's air offer requests API.
// Auth is a static bearer token; test mode is deterministic and free.
type Duffel struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewDuffel(token, baseURL string, httpClient *http.Client) *Duffel {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: duffelDefaultTimeout}
	}
	if baseURL == "" {
		baseURL = duffelDefaultBaseURL
	}
	return &Duffel{
		token:      token,
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (d *Duffel) Search(ctx context.Context, req SearchRequest) ([]Fare, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	body := map[string]any{
		"data": map[string]any{
			"slices": []map[string]any{{
				"origin":         req.Origin,
				"destination":    req.Destination,
				"departure_date": req.Date,
			}},
			"passengers": []map[string]any{{"type": "adult"}},
		},
	}

	resp, err := d.createOfferRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	if len(resp.Data.Offers) == 0 {
		return []Fare{}, nil
	}
	return mapDuffelOffers(resp.Data.Offers, req), nil
}

type offerRequestResponse struct {
	Data struct {
		ID     string        `json:"id"`
		Offers []duffelOffer `json:"offers"`
	} `json:"data"`
}

func (d *Duffel) createOfferRequest(ctx context.Context, body map[string]any) (*offerRequestResponse, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/air/offer_requests?return_offers=true", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.token)
	httpReq.Header.Set("Duffel-Version", duffelVersionHeader)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &providerDownError{}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &authError{}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &rateLimitedError{}
	}
	if resp.StatusCode == http.StatusBadRequest {
		return nil, &invalidInputError{}
	}
	if resp.StatusCode >= 500 {
		return nil, &providerDownError{}
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return nil, &providerDownError{}
	}

	var out offerRequestResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, &providerDownError{}
	}
	return &out, nil
}

type duffelOffer struct {
	ID           string        `json:"id"`
	TotalAmount  string        `json:"total_amount"`
	TotalCurrency string       `json:"total_currency"`
	ExpiresAt    string        `json:"expires_at"`
	Owner        duffelAirline `json:"owner"`
	Slices       []duffelSlice `json:"slices"`
}

type duffelAirline struct {
	Name      string `json:"name"`
	IATACode  string `json:"iata_code"`
}

type duffelSlice struct {
	Duration string         `json:"duration"`
	Segments []duffelSegment `json:"segments"`
}

type duffelSegment struct {
	OperatingCarrier        duffelAirline `json:"operating_carrier"`
	OperatingCarrierFlightNumber string  `json:"operating_carrier_flight_number"`
	DepartingAt string `json:"departing_at"`
	ArrivingAt  string `json:"arriving_at"`
	Origin      duffelPlace `json:"origin"`
	Destination duffelPlace `json:"destination"`
	Duration    string      `json:"duration"`
}

type duffelPlace struct {
	IATACode string `json:"iata_code"`
	Name     string `json:"name"`
}

func mapDuffelOffers(offers []duffelOffer, req SearchRequest) []Fare {
	out := make([]Fare, 0, len(offers))
	for _, o := range offers {
		if len(o.Slices) == 0 || len(o.Slices[0].Segments) == 0 {
			continue
		}
		slice := o.Slices[0]
		segments := slice.Segments
		first := segments[0]
		last := segments[len(segments)-1]

		airline := first.OperatingCarrier.IATACode
		if airline == "" {
			airline = o.Owner.IATACode
		}
		airlineName := first.OperatingCarrier.Name
		if airlineName == "" {
			airlineName = o.Owner.Name
		}

		dep := parseFlexibleTime(first.DepartingAt)
		if dep.IsZero() {
			continue
		}

		price := parsePriceCents(o.TotalAmount)
		dur := parseISODuration(slice.Duration)
		if dur == 0 {
			// sum segments if slice duration missing
			for _, s := range segments {
				dur += parseISODuration(s.Duration)
			}
		}

		// Flight number: combine carrier code + first segment's flight number.
		flightNum := first.OperatingCarrierFlightNumber
		displayFlight := ""
		if airline != "" && flightNum != "" {
			displayFlight = airline + " " + flightNum
		} else if flightNum != "" {
			displayFlight = flightNum
		}

		out = append(out, Fare{
			Origin:             req.Origin,
			Destination:        req.Destination,
			OriginAirport:      first.Origin.IATACode,
			DestinationAirport: last.Destination.IATACode,
			Airline:            firstNonEmpty(airline, airlineName),
			FlightNumber:       displayFlight,
			DepartureAt:        dep,
			Price:              price,
			Currency:           o.TotalCurrency,
			Duration:           dur,
			Transfers:          len(segments) - 1,
			Link:               "https://app.duffel.com/book/offers/" + o.ID,
		})
	}
	// Sort by price ascending so worker logs the cheapest first.
	sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

// parseFlexibleTime parses Duffel timestamps which may or may not include a timezone.
// Falls back to appending "Z" (UTC) for naive timestamps like "2026-10-15T10:50:00".
func parseFlexibleTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, s+"Z")
	return t
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func parsePriceCents(amount string) int {
	// Duffel returns decimal string like "345.67"; convert to integer cents.
	var f float64
	if _, err := fmt.Sscanf(amount, "%f", &f); err != nil {
		return 0
	}
	return int(math.Round(f * 100))
}

// parseISODuration parses ISO 8601 durations like "PT07H25M" or "PT2H26M".
// Returns 0 on parse error. Sufficient for flight durations (<24h).
func parseISODuration(s string) time.Duration {
	if s == "" {
		return 0
	}
	// Strip the PT prefix.
	if len(s) < 3 || s[:2] != "PT" {
		return 0
	}
	rest := s[2:]
	var hours, minutes int
	// Naive parser: look for H and M markers.
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case 'H':
			if v, err := atoiSafe(rest[:i]); err == nil {
				hours = v
			}
			rest = rest[i+1:]
			i = -1
		case 'M':
			if v, err := atoiSafe(rest[:i]); err == nil {
				minutes = v
			}
			rest = rest[i+1:]
			i = -1
		}
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
}

func atoiSafe(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
