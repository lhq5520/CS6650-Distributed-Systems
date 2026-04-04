package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

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

// parseURLList splits a comma-separated string into a list of URLs.
// e.g. "http://follower1:8080,http://follower2:8080" -> ["http://follower1:8080", "http://follower2:8080"]
func parseURLList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var urls []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			urls = append(urls, p)
		}
	}
	return urls
}

func main() {
	mode := getEnv("MODE", "leader")
	port := getEnv("PORT", "8080")

	mux := http.NewServeMux()

	switch mode {
	case "leader":
		followers := parseURLList(os.Getenv("FOLLOWERS"))
		w := getEnvInt("W", 5)
		r := getEnvInt("R", 1)

		node := &LeaderFollowerNode{
			store:     NewKVStore(),
			isLeader:  true,
			followers: followers,
			W:         w,
			R:         r,
		}

		// Client-facing endpoints
		mux.HandleFunc("/set", node.HandleSet)
		mux.HandleFunc("/get", node.HandleGet)
		mux.HandleFunc("/local_read", node.HandleLocalRead)

		// Internal endpoints (called by nobody for leader, but register anyway)
		mux.HandleFunc("/internal/set", node.HandleInternalSet)
		mux.HandleFunc("/internal/get", node.HandleInternalGet)

		log.Printf("Starting LEADER node on :%s (W=%d, R=%d, followers=%v)", port, w, r, followers)

	case "follower":
		leaderURL := getEnv("LEADER_URL", "http://leader:8080")
		w := getEnvInt("W", 5)
		r := getEnvInt("R", 1)

		node := &LeaderFollowerNode{
			store:     NewKVStore(),
			isLeader:  false,
			leaderURL: leaderURL,
			W:         w,
			R:         r,
		}

		// Client-facing endpoints
		mux.HandleFunc("/set", node.HandleSet)       // will forward to leader
		mux.HandleFunc("/get", node.HandleGet)
		mux.HandleFunc("/local_read", node.HandleLocalRead)

		// Internal endpoints (called by Leader during replication)
		mux.HandleFunc("/internal/set", node.HandleInternalSet)
		mux.HandleFunc("/internal/get", node.HandleInternalGet)

		log.Printf("Starting FOLLOWER node on :%s (leader=%s)", port, leaderURL)

	case "leaderless":
		peers := parseURLList(os.Getenv("PEERS"))

		node := &LeaderlessNode{
			store: NewKVStore(),
			peers: peers,
		}

		// Client-facing endpoints
		mux.HandleFunc("/set", node.HandleSet)
		mux.HandleFunc("/get", node.HandleGet)
		mux.HandleFunc("/local_read", node.HandleLocalRead)

		// Internal endpoint (called by Write Coordinator during replication)
		mux.HandleFunc("/internal/set", node.HandleInternalSet)

		log.Printf("Starting LEADERLESS node on :%s (peers=%v)", port, peers)

	default:
		log.Fatalf("Unknown MODE: %s (must be leader, follower, or leaderless)", mode)
	}

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	log.Fatal(http.ListenAndServe(":"+port, mux))
}
