package fareprovider

import (
	"context"
	"fmt"
	"time"
)

type SearchRequest struct {
	Origin      string
	Destination string
	Date        string
}

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
	// Cached is true when this fare was served from the Redis fare cache
	// rather than fetched fresh from the provider.
	Cached bool
}

type FareProvider interface {
	Search(ctx context.Context, req SearchRequest) ([]Fare, error)
}

type invalidInputError struct{}

func (e *invalidInputError) Error() string  { return "invalid input" }
func (e *invalidInputError) Unwrap() error { return fmt.Errorf("invalid input: malformed request") }
func (e *invalidInputError) Is(target error) bool {
	_, ok := target.(*invalidInputError)
	return ok
}

type authError struct{}

func (e *authError) Error() string  { return "authentication failed" }
func (e *authError) Unwrap() error { return fmt.Errorf("authentication failed: rejected by provider") }
func (e *authError) Is(target error) bool {
	_, ok := target.(*authError)
	return ok
}

type rateLimitedError struct{}

func (e *rateLimitedError) Error() string  { return "rate limited" }
func (e *rateLimitedError) Unwrap() error { return fmt.Errorf("rate limited: provider quota exceeded") }
func (e *rateLimitedError) Is(target error) bool {
	_, ok := target.(*rateLimitedError)
	return ok
}

type providerDownError struct{}

func (e *providerDownError) Error() string  { return "provider unavailable" }
func (e *providerDownError) Unwrap() error { return fmt.Errorf("provider unavailable: request could not complete") }
func (e *providerDownError) Is(target error) bool {
	_, ok := target.(*providerDownError)
	return ok
}

var (
	ErrInvalidInput  = &invalidInputError{}
	ErrAuth          = &authError{}
	ErrRateLimited   = &rateLimitedError{}
	ErrProviderDown  = &providerDownError{}
)
