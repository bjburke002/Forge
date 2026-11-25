package api

import (
	"encoding/json"
	"net/http"
	"time"
)

func LedgerHandler(url string) http.HandlerFunc {
	c := http.Client{Timeout: 3 * time.Second}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if url == "" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "LEDGER_URL not set"})
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), "GET", url, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "LEDGER_URL invalid"})
			return
		}

		resp, err := c.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"status": "bad gateway"})
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(map[string]string{"status": resp.Status})
		return
	}
}
