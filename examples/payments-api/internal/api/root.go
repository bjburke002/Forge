package api

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
)

// RootHandler handles all traffic that does not fall on the /health/* endpoints
func RootHandler(er float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		randFloat := rand.Float64()
		// Passing in an ERROR_RATE env to mock failures at that rate
		// Compare against a random float between 0-1 to determine which return HTTP/500
		if er > randFloat {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "internal server error"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "hello world!"})

	}
}
