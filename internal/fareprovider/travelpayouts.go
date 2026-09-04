package fareprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://api.travelpayouts.com/aviasales"
	defaultUserAgent  = "flight-circle-search/1.0"
	defaultTimeout    = 10 * time.Second
	maxResponseBytes  = 1 << 20
)

type Travelpayouts struct {
	token     string
	baseURL   string
	httpClient *http.Client
}

func NewTravelpayouts(token, baseURL string, httpClient *http.Client) *Travelpayouts {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Travelpayouts{
		token:      token,
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (t *Travelpayouts) Search(ctx context.Context, req SearchRequest) ([]Fare, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	fares, err := t.searchAtDate(ctx, req.Origin, req.Destination, req.Date)
	if err != nil {
		return nil, err
	}
	if len(fares) > 0 {
		return fares, nil
	}

	month := req.Date[:7]
	fares, err = t.searchAtDate(ctx, req.Origin, req.Destination, month)
	if err != nil {
		if errors.Is(err, ErrProviderDown) {
			return []Fare{}, nil
		}
		return nil, err
	}
	if len(fares) == 0 {
		return []Fare{}, nil
	}

	sort.Slice(fares, func(i, j int) bool {
		return fares[i].Price < fares[j].Price
	})
	return fares[:1], nil
}

func (t *Travelpayouts) searchAtDate(ctx context.Context, origin, destination, departureAt string) ([]Fare, error) {
	u, err := url.Parse(t.baseURL + "/v3/prices_for_dates")
	if err != nil {
		return nil, fmt.Errorf("base url: %w", err)
	}
	q := u.Query()
	q.Set("origin", origin)
	q.Set("destination", destination)
	q.Set("departure_at", departureAt)
	q.Set("one_way", "true")
	q.Set("currency", "usd")
	q.Set("sorting", "price")
	q.Set("limit", "30")
	q.Set("token", t.token)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &providerDownError{}
	}
	defer resp.Body.Close()

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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, &providerDownError{}
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, &providerDownError{}
	}
	if _, hasData := probe["data"]; !hasData {
		return nil, &providerDownError{}
	}

	var tpResp tpResponse
	if err := json.Unmarshal(body, &tpResp); err != nil {
		return nil, &providerDownError{}
	}

	if !tpResp.Success {
		return nil, &providerDownError{}
	}

	if tpResp.Data == nil {
		return []Fare{}, nil
	}

	if strings.ToLower(tpResp.Currency) != "usd" {
		return nil, &providerDownError{}
	}

	return mapFares(*tpResp.Data)
}

type tpResponse struct {
	Success  bool       `json:"success"`
	Data     *[]tpFlight `json:"data"`
	Currency string     `json:"currency"`
	Error    string     `json:"error,omitempty"`
}

type tpFlight struct {
	OriginAirport       string  `json:"origin_airport"`
	DestinationAirport  string  `json:"destination_airport"`
	Origin              string  `json:"origin"`
	Destination         string  `json:"destination"`
	Airline             string  `json:"airline"`
	FlightNumber        string  `json:"flight_number"`
	DepartureAt         string  `json:"departure_at"`
	Price               float64 `json:"price"`
	Duration            int     `json:"duration"`
	DurationTo          int     `json:"duration_to"`
	DurationBack        int     `json:"duration_back"`
	Transfers           int     `json:"transfers"`
	ReturnTransfers     int     `json:"return_transfers"`
	Link                string  `json:"link"`
}

func mapFares(flights []tpFlight) ([]Fare, error) {
	if flights == nil {
		return []Fare{}, nil
	}
	fares := make([]Fare, 0, len(flights))
	for _, f := range flights {
		if f.OriginAirport == "" || f.DestinationAirport == "" || f.DepartureAt == "" || f.Link == "" {
			return nil, &providerDownError{}
		}
		dep, err := time.Parse(time.RFC3339, f.DepartureAt)
		if err != nil {
			return nil, &providerDownError{}
		}
		fares = append(fares, Fare{
			Origin:              f.Origin,
			Destination:         f.Destination,
			OriginAirport:       f.OriginAirport,
			DestinationAirport:  f.DestinationAirport,
			Airline:             f.Airline,
			FlightNumber:        f.FlightNumber,
			DepartureAt:         dep,
			Price:               int(math.Round(f.Price * 100)),
			Currency:            "USD",
			Duration:            time.Duration(f.Duration) * time.Minute,
			Transfers:           f.Transfers,
			Link:                f.Link,
		})
	}
	return fares, nil
}

var iataRegex = regexp.MustCompile(`^[A-Z]{3}$`)
var dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validateRequest(req SearchRequest) error {
	if !iataRegex.MatchString(req.Origin) {
		return fmt.Errorf("origin %q: %w", req.Origin, ErrInvalidInput)
	}
	if !iataRegex.MatchString(req.Destination) {
		return fmt.Errorf("destination %q: %w", req.Destination, ErrInvalidInput)
	}
	if !dateRegex.MatchString(req.Date) {
		return fmt.Errorf("date %q: %w", req.Date, ErrInvalidInput)
	}
	_, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return fmt.Errorf("date %q: %w", req.Date, ErrInvalidInput)
	}
	return nil
}


