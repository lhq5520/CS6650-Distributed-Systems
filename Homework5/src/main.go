package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// ==================== Models ====================

type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ==================== In-Memory Store ====================

type ProductStore struct {
	mu       sync.RWMutex
	products map[int]*Product
}

func NewProductStore() *ProductStore {
	return &ProductStore{
		products: make(map[int]*Product),
	}
}

func (s *ProductStore) Get(id int) (*Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

func (s *ProductStore) Set(id int, p *Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[id] = p
}

// ==================== Middleware ====================

func recoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC recovered: %v", err)
				writeError(w, http.StatusInternalServerError,
					"INTERNAL_ERROR", "Internal server error",
					fmt.Sprintf("%v", err))
			}
		}()
		next(w, r)
	}
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://127.0.0.1:5500" || origin == "http://localhost:5500" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// ==================== Router & Handlers ====================

var store = NewProductStore()

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/products/", corsMiddleware(recoveryMiddleware(handleProducts)))

	port := ":5173"
	log.Printf("Product API server starting on %s", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func handleProducts(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Debug-Panic") == "1" {
		panic("debug panic")
	}

	path := strings.TrimPrefix(r.URL.Path, "/products/")
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Product ID is required", "")
		return
	}

	productID, err := strconv.Atoi(parts[0])
	if err != nil || productID < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid product ID",
			"Product ID must be a positive integer")
		return
	}

	switch {
	// GET /products/{productId}
	case len(parts) == 1 && r.Method == http.MethodGet:
		handleGetProduct(w, productID)

	// POST /products/{productId}/details
	case len(parts) == 2 && parts[1] == "details" && r.Method == http.MethodPost:
		handleAddProductDetails(w, r, productID)

	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
	}
}

// GET /products/{productId} → 200 / 404 / 500
func handleGetProduct(w http.ResponseWriter, productID int) {
	product, found := store.Get(productID)
	if !found {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Product not found",
			fmt.Sprintf("No product found with ID %d", productID))
		return
	}
	writeJSON(w, http.StatusOK, product)
}

// POST /products/{productId}/details → 204 / 400 / 404 / 500
// Spec: "Add or update detailed information for a specific product"
// We treat this as an upsert. 404 is returned when body product_id != path productId.
func handleAddProductDetails(w http.ResponseWriter, r *http.Request, productID int) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT",
			"Invalid JSON in request body", err.Error())
		return
	}

	// 404: body product_id doesn't match the path — "product not found"
	if product.ProductID != productID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Product not found",
			fmt.Sprintf("product_id in body (%d) does not match path (%d)",
				product.ProductID, productID))
		return
	}

	// 400: validate all required fields
	if errMsg := validateProduct(&product); errMsg != "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT",
			"The provided input data is invalid", errMsg)
		return
	}

	// 204: success
	store.Set(productID, &product)
	w.WriteHeader(http.StatusNoContent)
}

// ==================== Validation ====================

func validateProduct(p *Product) string {
	if p.ProductID < 1 {
		return "product_id must be a positive integer"
	}
	if p.SKU == "" {
		return "sku is required"
	}
	if len(p.SKU) > 100 {
		return "sku must be at most 100 characters"
	}
	if p.Manufacturer == "" {
		return "manufacturer is required"
	}
	if len(p.Manufacturer) > 200 {
		return "manufacturer must be at most 200 characters"
	}
	if p.CategoryID < 1 {
		return "category_id must be a positive integer"
	}
	if p.Weight < 0 {
		return "weight must be a non-negative integer"
	}
	if p.SomeOtherID < 1 {
		return "some_other_id must be a positive integer"
	}
	return ""
}

// ==================== Response Helpers ====================

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, errCode, message, details string) {
	writeJSON(w, status, ErrorResponse{Error: errCode, Message: message, Details: details})
}
