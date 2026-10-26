package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

// File is a JSON struct for the dependencies file we are feeding this program to mock dependency failure
type File struct {
	ServiceName  string       `json:"serviceName"`
	Dependencies []Dependency `json:"dependencies"`
}

// Dependency is the JSON struct for dependencies read from the file
type Dependency struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Healthy bool   `json:"healthy"`
}

// LivenessHandler handles the app's basic liveness probe and returns http/200 if it's up
func LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// ReadinessHandler handles readiness evaluation for the mocked dependencies. It returns a 5xx error code for
// any in an un-ready state and a HTTP/200 if all are set to healthy
func ReadinessHandler(path string) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// Runs on every request and uses the path env from the outer function
		w.Header().Set("Content-Type", "application/json")

		// Read and parse the file at PATH
		fileBytes, err := os.ReadFile(path)

		// Make sure the file is readable
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "dependency file unreadable"})
			slog.Error("Reading dependency file", "path", path, "error", err)
			return
		}
		// Checking for valid file
		var file File
		if err := json.Unmarshal(fileBytes, &file); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "dependency file invalid"})
			slog.Error("Checking file validity", "error", err)
			return
		}
		// Grab all unhealthy services and return them
		var unhealthy []Dependency
		for _, dep := range file.Dependencies {
			if !dep.Healthy {
				unhealthy = append(unhealthy, dep)
			}
		}
		if len(unhealthy) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]any{
				"status":    "not ready",
				"unhealthy": unhealthy,
			})
			return
		}

		// No errors: Write HTTP/200
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}
