package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ==================== Models ====================

type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

type Recommendation struct {
	ProductID   int     `json:"product_id"`
	ProductName string  `json:"product_name"`
	Score       float64 `json:"score"`
}

type SearchResponse struct {
	Products        []Product        `json:"products"`
	Recommendations []Recommendation `json:"recommendations"`
	TotalFound      int              `json:"total_found"`
	SearchTime      string           `json:"search_time"`
	RecStatus       string           `json:"rec_status"` // "live", "fallback", "circuit_open", "timeout", "bulkhead_rejected"
}

// ==================== Data Store ====================

var store sync.Map
var brands = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}
var categories = []string{"Electronics", "Books", "Home", "Sports", "Clothing"}

func generateProducts() {
	for i := 0; i < 100000; i++ {
		brand := brands[i%len(brands)]
		category := categories[i%len(categories)]
		store.Store(i, Product{
			ID:          i + 1,
			Name:        fmt.Sprintf("Product %s %d", brand, i+1),
			Category:    category,
			Description: fmt.Sprintf("Description for product %d", i+1),
			Brand:       brand,
		})
	}
	log.Println("Generated 100,000 products")
}

// ==================== Circuit Breaker ====================

type CircuitBreaker struct {
	mu               sync.Mutex
	state            string // "closed", "open", "half_open"
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
	lastFailure      time.Time

	// Metrics
	TotalRequests  atomic.Int64
	TotalFailures  atomic.Int64
	TotalOpens     atomic.Int64
	TotalFallbacks atomic.Int64
}

func NewCircuitBreaker(failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            "closed",
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		openTimeout:      openTimeout,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.TotalRequests.Add(1)

	switch cb.state {
	case "closed":
		return true
	case "open":
		if time.Since(cb.lastFailure) > cb.openTimeout {
			cb.state = "half_open"
			cb.successCount = 0
			log.Println("[CIRCUIT BREAKER] State: OPEN -> HALF_OPEN (trying again)")
			return true
		}
		cb.TotalFallbacks.Add(1)
		return false
	case "half_open":
		return true
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "half_open" {
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = "closed"
			cb.failureCount = 0
			log.Println("[CIRCUIT BREAKER] State: HALF_OPEN -> CLOSED (recovered!)")
		}
	} else {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.TotalFailures.Add(1)
	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.failureCount >= cb.failureThreshold {
		if cb.state != "open" {
			cb.state = "open"
			cb.TotalOpens.Add(1)
			log.Printf("[CIRCUIT BREAKER] State: -> OPEN (failures=%d, threshold=%d)", cb.failureCount, cb.failureThreshold)
		}
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ==================== Bulkhead ====================

type Bulkhead struct {
	sem            chan struct{}
	maxConcurrent  int
	TotalRejected  atomic.Int64
	TotalAdmitted  atomic.Int64
}

func NewBulkhead(maxConcurrent int) *Bulkhead {
	return &Bulkhead{
		sem:           make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

func (b *Bulkhead) TryAcquire() bool {
	select {
	case b.sem <- struct{}{}:
		b.TotalAdmitted.Add(1)
		return true
	default:
		b.TotalRejected.Add(1)
		return false
	}
}

func (b *Bulkhead) Release() {
	<-b.sem
}

// ==================== Config ====================

var (
	resilience    bool
	recServiceURL string
	cb            *CircuitBreaker
	bulkhead      *Bulkhead
	httpClient    *http.Client // with timeout for fail-fast
	httpClientRaw *http.Client // no timeout for broken version
)

// Fallback recommendations when downstream is unavailable
var fallbackRecs = []Recommendation{
	{ProductID: 1, ProductName: "Popular: Product Alpha 1", Score: 0.90},
	{ProductID: 2, ProductName: "Popular: Product Beta 2", Score: 0.85},
	{ProductID: 3, ProductName: "Popular: Product Gamma 3", Score: 0.80},
}

// ==================== Metrics ====================

type Metrics struct {
	TotalSearches    atomic.Int64
	SuccessfulRecs   atomic.Int64
	FailedRecs       atomic.Int64
	TimeoutRecs      atomic.Int64
	FallbackRecs     atomic.Int64
}

var metrics Metrics

// ==================== Handlers ====================

func getRecommendations() ([]Recommendation, string) {
	metrics.TotalSearches.Add(1)

	if !resilience {
		// ===== NO RESILIENCE: just call downstream, wait forever =====
		resp, err := httpClientRaw.Get(recServiceURL + "/recommendations")
		if err != nil {
			metrics.FailedRecs.Add(1)
			return nil, "error"
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			metrics.FailedRecs.Add(1)
			return nil, "error"
		}

		var recs []Recommendation
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &recs)
		metrics.SuccessfulRecs.Add(1)
		return recs, "live"
	}

	// ===== WITH RESILIENCE =====

	// Pattern 1: Circuit Breaker — don't even try if circuit is open
	if !cb.Allow() {
		metrics.FallbackRecs.Add(1)
		return fallbackRecs, "circuit_open"
	}

	// Pattern 2: Bulkhead — limit concurrent downstream calls
	if !bulkhead.TryAcquire() {
		metrics.FallbackRecs.Add(1)
		cb.RecordFailure()
		return fallbackRecs, "bulkhead_rejected"
	}
	defer bulkhead.Release()

	// Pattern 3: Fail Fast — timeout on downstream call
	resp, err := httpClient.Get(recServiceURL + "/recommendations")
	if err != nil {
		metrics.TimeoutRecs.Add(1)
		cb.RecordFailure()
		return fallbackRecs, "timeout"
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		metrics.FailedRecs.Add(1)
		cb.RecordFailure()
		return fallbackRecs, "fallback"
	}

	var recs []Recommendation
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &recs)

	cb.RecordSuccess()
	metrics.SuccessfulRecs.Add(1)
	return recs, "live"
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	query := strings.ToLower(r.URL.Query().Get("q"))

	// Search products (same as HW6 — check first 100)
	var results []Product
	totalFound := 0
	checked := 0

	for i := 0; i < 100000 && checked < 100; i++ {
		val, ok := store.Load(i)
		if !ok {
			continue
		}
		p := val.(Product)
		checked++
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Category), query) {
			totalFound++
			if len(results) < 20 {
				results = append(results, p)
			}
		}
	}

	if results == nil {
		results = []Product{}
	}

	// Get recommendations from downstream
	recs, recStatus := getRecommendations()
	if recs == nil {
		recs = []Recommendation{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SearchResponse{
		Products:        results,
		Recommendations: recs,
		TotalFound:      totalFound,
		SearchTime:      time.Since(start).String(),
		RecStatus:       recStatus,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"resilience_enabled": resilience,
		"total_searches":     metrics.TotalSearches.Load(),
		"successful_recs":    metrics.SuccessfulRecs.Load(),
		"failed_recs":        metrics.FailedRecs.Load(),
		"timeout_recs":       metrics.TimeoutRecs.Load(),
		"fallback_recs":      metrics.FallbackRecs.Load(),
	}

	if resilience {
		data["circuit_breaker"] = map[string]interface{}{
			"state":           cb.State(),
			"total_requests":  cb.TotalRequests.Load(),
			"total_failures":  cb.TotalFailures.Load(),
			"total_opens":     cb.TotalOpens.Load(),
			"total_fallbacks": cb.TotalFallbacks.Load(),
		}
		data["bulkhead"] = map[string]interface{}{
			"max_concurrent": bulkhead.maxConcurrent,
			"total_admitted": bulkhead.TotalAdmitted.Load(),
			"total_rejected": bulkhead.TotalRejected.Load(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// ==================== Main ====================

func main() {
	generateProducts()

	// Config from env
	resilience = os.Getenv("RESILIENCE") == "true"
	recServiceURL = os.Getenv("REC_SERVICE_URL")
	if recServiceURL == "" {
		recServiceURL = "http://recommendation:8081"
	}

	// Fail-fast HTTP client: 500ms timeout
	httpClient = &http.Client{Timeout: 500 * time.Millisecond}

	// Raw HTTP client: no timeout (for broken version)
	httpClientRaw = &http.Client{Timeout: 0} // no timeout = wait forever

	// Circuit breaker: open after 5 failures, recover after 3 successes, retry after 15s
	cb = NewCircuitBreaker(5, 3, 15*time.Second)

	// Bulkhead: max 10 concurrent calls to downstream
	bulkhead = NewBulkhead(10)

	mode := "NO RESILIENCE (vulnerable)"
	if resilience {
		mode = "WITH RESILIENCE (fail-fast + circuit breaker + bulkhead)"
	}
	log.Printf("Search service starting | resilience=%s | downstream=%s", mode, recServiceURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/products/search", handleSearch)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/metrics", handleMetrics)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
