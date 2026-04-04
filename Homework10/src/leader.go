package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

// LeaderFollowerNode represents a node in the Leader-Follower cluster.
// It can act as either a Leader or a Follower depending on configuration.
type LeaderFollowerNode struct {
	store     *KVStore
	isLeader  bool
	followers []string // URLs of follower nodes, e.g. ["http://follower1:8080"]
	leaderURL string   // URL of the leader node (used by followers to forward writes)
	W         int      // Number of nodes that must confirm a write
	R         int      // Number of nodes to read from
}

// SetRequest is the JSON body for set/write operations.
type SetRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int    `json:"version,omitempty"` // only used for internal replication
}

// GetResponse is the JSON body returned for read operations.
type GetResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int    `json:"version"`
}

// --- Client-facing endpoints ---

// HandleSet processes a client write request.
// If this node is the Leader: store locally, then replicate to followers.
// If this node is a Follower: forward the request to the Leader.
func (n *LeaderFollowerNode) HandleSet(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	if !n.isLeader {
		// Follower: forward to Leader
		n.forwardToLeader(w, req)
		return
	}

	// Leader: assign version and store locally
	version := n.store.SetAndIncrement(req.Key, req.Value)

	// Replicate to followers
	// W is how many nodes must confirm before we respond to the client.
	// W includes the Leader itself, so we need W-1 follower confirmations.
	needed := n.W - 1 // Leader already stored it, so subtract 1
	if needed <= 0 {
		// W=1: Leader only, respond immediately. Replicate in background.
		go n.replicateToFollowers(req.Key, req.Value, version, len(n.followers))
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(GetResponse{Key: req.Key, Value: req.Value, Version: version})
		return
	}

	// W>1: must wait for some followers to confirm
	acks := n.replicateToFollowers(req.Key, req.Value, version, needed)
	if acks < needed {
		http.Error(w, "failed to replicate to enough followers", http.StatusInternalServerError)
		return
	}

	// If we still have un-replicated followers (e.g. W=3 but N=5),
	// replicate to the rest in background
	remaining := len(n.followers) - needed
	if remaining > 0 {
		go n.replicateToFollowers(req.Key, req.Value, version, len(n.followers))
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(GetResponse{Key: req.Key, Value: req.Value, Version: version})
}

// HandleGet processes a client read request.
// Depending on R, it reads from 1 or more nodes and returns the newest version.
func (n *LeaderFollowerNode) HandleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if n.R == 1 {
		// R=1: just read from this node (could be Leader or any single node)
		entry, ok := n.store.Get(key)
		if !ok {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(GetResponse{Key: key, Value: entry.Value, Version: entry.Version})
		return
	}

	// R>1: read from multiple nodes and return the newest version
	results := n.readFromMultipleNodes(key, n.R)
	if len(results) == 0 {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	// Pick the entry with the highest version
	sort.Slice(results, func(i, j int) bool {
		return results[i].Version > results[j].Version
	})
	best := results[0]
	json.NewEncoder(w).Encode(GetResponse{Key: key, Value: best.Value, Version: best.Version})
}

// HandleLocalRead returns the value stored locally on THIS node only.
// This is a "sneaky" testing endpoint to observe inconsistency during replication.
func (n *LeaderFollowerNode) HandleLocalRead(w http.ResponseWriter, r *http.Request) {
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

// --- Internal endpoints (node-to-node communication) ---

// HandleInternalSet is called by the Leader to replicate data to this Follower.
// The Follower sleeps 100ms to simulate storage delay, then stores the data.
func (n *LeaderFollowerNode) HandleInternalSet(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Simulate storage delay on follower (100ms as required by assignment)
	time.Sleep(100 * time.Millisecond)

	n.store.Set(req.Key, req.Value, req.Version)
	w.WriteHeader(http.StatusOK)
}

// HandleInternalGet is called by the Leader when R>1 to read from this Follower.
// The Follower sleeps 50ms to simulate read delay, then returns the data.
func (n *LeaderFollowerNode) HandleInternalGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	// Simulate read delay on follower (50ms as required by assignment)
	time.Sleep(50 * time.Millisecond)

	entry, ok := n.store.Get(key)
	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(GetResponse{Key: key, Value: entry.Value, Version: entry.Version})
}

// --- Replication logic ---

// replicateToFollowers sends a write to followers and waits for 'needed' confirmations.
// The Leader sleeps 200ms after each follower update (as required by assignment).
// Returns the number of successful acknowledgments.
func (n *LeaderFollowerNode) replicateToFollowers(key, value string, version, needed int) int {
	if needed <= 0 || len(n.followers) == 0 {
		return 0
	}

	type ack struct{ ok bool }
	ch := make(chan ack, len(n.followers))

	var wg sync.WaitGroup
	for i, followerURL := range n.followers {
		if i >= needed {
			break
		}
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

			// Leader sleeps 200ms per follower update (as required by assignment)
			time.Sleep(200 * time.Millisecond)

			ch <- ack{ok: resp.StatusCode == http.StatusOK}
		}(followerURL)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Count successful acks
	acks := 0
	for a := range ch {
		if a.ok {
			acks++
		}
	}
	return acks
}

// readFromMultipleNodes reads a key from this node + remote followers.
// Returns up to 'count' results.
func (n *LeaderFollowerNode) readFromMultipleNodes(key string, count int) []KVEntry {
	var results []KVEntry
	var mu sync.Mutex

	// Read from local store first
	if entry, ok := n.store.Get(key); ok {
		results = append(results, entry)
	}

	if count <= 1 {
		return results
	}

	// Read from remote followers in parallel
	var wg sync.WaitGroup
	remaining := count - 1 // we already read locally
	for i := 0; i < len(n.followers) && i < remaining; i++ {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			resp, err := http.Get(url + "/internal/get?key=" + key)
			if err != nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					resp.Body.Close()
				}
				return
			}
			defer resp.Body.Close()
			var entry GetResponse
			if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
				return
			}
			mu.Lock()
			results = append(results, KVEntry{Value: entry.Value, Version: entry.Version})
			mu.Unlock()
		}(n.followers[i])
	}
	wg.Wait()
	return results
}

// forwardToLeader forwards a write request from a Follower to the Leader.
func (n *LeaderFollowerNode) forwardToLeader(w http.ResponseWriter, req SetRequest) {
	body, _ := json.Marshal(req)
	resp, err := http.Post(n.leaderURL+"/set", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to forward to leader", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Pass through the Leader's response
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
