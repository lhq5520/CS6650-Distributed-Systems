package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
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

type SearchResponse struct {
	Products   []Product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time"`
}

// ==================== Data ====================

var store sync.Map // key: int, value: Product

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

// ==================== Handlers ====================

func handleSearch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	query := strings.ToLower(r.URL.Query().Get("q"))

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SearchResponse{
		Products:   results,
		TotalFound: totalFound,
		SearchTime: time.Since(start).String(),
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// ==================== Main ====================

func main() {
	generateProducts()

	mux := http.NewServeMux()
	mux.HandleFunc("/products/search", handleSearch)
	mux.HandleFunc("/health", handleHealth)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}