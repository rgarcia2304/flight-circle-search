package auth

import "errors"

var (
	ErrRateLimited = errors.New("rate limited")
	ErrNotFound    = errors.New("not found")
)
