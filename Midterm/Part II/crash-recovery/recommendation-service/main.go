package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// Recommendation service that can simulate failures
// Control via environment variables:
//   FAILURE_MODE=none|slow|error|crash
//   SLOW_MIN_MS=3000   (min delay in ms)
//   SLOW_MAX_MS=8000   (max delay in ms)
//   ERROR_RATE=0.8     (probability of error in "error" mode)

type Recommendation struct {
	ProductID   int    `json:"product_id"`
	ProductName string `json:"product_name"`
	Score       float64 `json:"score"`
}

var (
	failureMode string
	slowMinMs   int
	slowMaxMs   int
	errorRate   float64
	requestCount atomic.Int64
)

func init() {
	failureMode = getEnv("FAILURE_MODE", "none")
	slowMinMs = getEnvInt("SLOW_MIN_MS", 3000)
	slowMaxMs = getEnvInt("SLOW_MAX_MS", 8000)
	errorRate = getEnvFloat("ERROR_RATE", 0.8)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return f
		}
	}
	return fallback
}

func handleRecommendations(w http.ResponseWriter, r *http.Request) {
	count := requestCount.Add(1)

	switch failureMode {
	case "slow":
		// Simulate slow responses
		delay := slowMinMs + rand.Intn(slowMaxMs-slowMinMs)
		log.Printf("[req #%d] Simulating slow response: %dms delay", count, delay)
		time.Sleep(time.Duration(delay) * time.Millisecond)

	case "error":
		// Randomly return 500 errors
		if rand.Float64() < errorRate {
			log.Printf("[req #%d] Simulating error (rate=%.0f%%)", count, errorRate*100)
			http.Error(w, `{"error":"internal service error"}`, http.StatusInternalServerError)
			return
		}

	case "crash":
		// Slow responses that get progressively worse, then crash
		if count > 50 {
			log.Printf("[req #%d] Simulating crash! Exiting...", count)
			os.Exit(1)
		}
		delay := int(count) * 100 // gets slower over time
		log.Printf("[req #%d] Degrading: %dms delay", count, delay)
		time.Sleep(time.Duration(delay) * time.Millisecond)

	default:
		// "none" - healthy mode
		log.Printf("[req #%d] Healthy response", count)
	}

	// Return recommendations
	recs := []Recommendation{
		{ProductID: rand.Intn(100000), ProductName: "Recommended Alpha", Score: 0.95},
		{ProductID: rand.Intn(100000), ProductName: "Recommended Beta", Score: 0.87},
		{ProductID: rand.Intn(100000), ProductName: "Recommended Gamma", Score: 0.82},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recs)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Allow runtime mode change via POST /mode?mode=slow
func handleModeChange(w http.ResponseWriter, r *http.Request) {
	newMode := r.URL.Query().Get("mode")
	if newMode == "" {
		http.Error(w, "missing ?mode= parameter", http.StatusBadRequest)
		return
	}
	oldMode := failureMode
	failureMode = newMode
	requestCount.Store(0)
	log.Printf("Mode changed: %s -> %s", oldMode, newMode)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"old_mode": oldMode,
		"new_mode": newMode,
		"status":   "ok",
	})
}

func main() {
	log.Printf("Recommendation service starting | mode=%s", failureMode)

	mux := http.NewServeMux()
	mux.HandleFunc("/recommendations", handleRecommendations)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/mode", handleModeChange)

	log.Fatal(http.ListenAndServe(":8081", mux))
}
