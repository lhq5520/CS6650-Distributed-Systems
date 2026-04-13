package handler

import "net/http"

// Pre-serialized JSON for zero-allocation health response
var healthBody = []byte(`{"status":"ok"}`)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(healthBody)
}
