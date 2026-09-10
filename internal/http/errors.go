package http

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrUnauthorized is returned/used when a request has no valid session.
var ErrUnauthorized = errors.New("unauthorized")

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
