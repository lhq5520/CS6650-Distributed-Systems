package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// healthHandler - ECS health check
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// syncHandler - Phase 1: Synchronous order processing
// Simulates 3-second payment verification using a buffered channel to truly block throughput
func syncHandler(w http.ResponseWriter, r *http.Request) {
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	order.OrderID = generateID()
	order.CreatedAt = time.Now()
	order.Status = "processing"

	// Simulate payment verification bottleneck.
	// We use a buffered channel of size 1 to truly block throughput
	// (a plain time.Sleep would not block the OS thread in Go).
	done := make(chan struct{}, 1)
	go func() {
		time.Sleep(3 * time.Second)
		done <- struct{}{}
	}()
	<-done // block until payment "completes"

	order.Status = "completed"

	log.Printf("[SYNC] Order %s completed for customer %d", order.OrderID, order.CustomerID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(order)
}

// asyncHandler - Phase 3: Asynchronous order processing
// Publishes to SNS immediately and returns 202 Accepted
func asyncHandler(snsPublisher SNSPublisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var order Order
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		order.OrderID = generateID()
		order.CreatedAt = time.Now()
		order.Status = "pending"

		if err := snsPublisher.Publish(r.Context(), order); err != nil {
			log.Printf("[ASYNC] Failed to publish order %s: %v", order.OrderID, err)
			http.Error(w, `{"error":"failed to queue order"}`, http.StatusInternalServerError)
			return
		}

		log.Printf("[ASYNC] Order %s queued for customer %d", order.OrderID, order.CustomerID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted) // 202
		json.NewEncoder(w).Encode(map[string]string{
			"order_id": order.OrderID,
			"status":   "pending",
			"message":  "Order queued for processing",
		})
	}
}
