package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// LeaderlessNode represents a node in the Leaderless cluster.
// Every node is equal — any node can handle both reads and writes.
// When a node receives a write, it becomes the Write Coordinator for that request.
type LeaderlessNode struct {
	store *KVStore
	peers []string // URLs of all other nodes, e.g. ["http://node2:8080", ...]
}

// HandleSet processes a client write request.
// This node becomes the Write Coordinator:
// 1. Store locally and assign a version number
// 2. Replicate to ALL other nodes (W=N)
// 3. Only return 201 after ALL nodes confirm
func (n *LeaderlessNode) HandleSet(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	// This node is the Write Coordinator: assign version locally
	version := n.store.SetAndIncrement(req.Key, req.Value)

	// Replicate to ALL peers (W=N, so we must wait for every node)
	acks := n.replicateToPeers(req.Key, req.Value, version)

	if acks < len(n.peers) {
		http.Error(w, "failed to replicate to all nodes", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(GetResponse{Key: req.Key, Value: req.Value, Version: version})
}

// HandleGet processes a client read request.
// R=1: just return this node's local value. Simple and fast,
// but may return stale data if a concurrent write hasn't reached this node yet.
func (n *LeaderlessNode) HandleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	entry, ok := n.store.Get(key)
	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(GetResponse{Key: key, Value: entry.Value, Version: entry.Version})
}

// HandleLocalRead is the same as HandleGet for Leaderless (R=1 means we always read locally).
// Included for API compatibility with the Leader-Follower database.
func (n *LeaderlessNode) HandleLocalRead(w http.ResponseWriter, r *http.Request) {
	n.HandleGet(w, r)
}

// --- Internal endpoint ---

// HandleInternalSet is called by a Write Coordinator to replicate data to this node.
// Sleeps 100ms to simulate storage delay.
func (n *LeaderlessNode) HandleInternalSet(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Simulate storage delay (100ms)
	time.Sleep(100 * time.Millisecond)

	n.store.Set(req.Key, req.Value, req.Version)
	w.WriteHeader(http.StatusOK)
}

// --- Replication logic ---

// replicateToPeers sends a write to ALL peers and waits for ALL to confirm (W=N).
// The coordinator sleeps 200ms after each peer update.
// Returns the number of successful acknowledgments.
func (n *LeaderlessNode) replicateToPeers(key, value string, version int) int {
	type ack struct{ ok bool }
	ch := make(chan ack, len(n.peers))

	var wg sync.WaitGroup
	for _, peerURL := range n.peers {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			body, _ := json.Marshal(SetRequest{
				Key:     key,
				Value:   value,
				Version: version,
			})

			resp, err := http.Post(url+"/internal/set", "application/json", bytes.NewReader(body))
			if err != nil {
				ch <- ack{ok: false}
				return
			}
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body)

			// Coordinator sleeps 200ms per peer update
			time.Sleep(200 * time.Millisecond)

			ch <- ack{ok: resp.StatusCode == http.StatusOK}
		}(peerURL)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	acks := 0
	for a := range ch {
		if a.ok {
			acks++
		}
	}
	return acks
}
