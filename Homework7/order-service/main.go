package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

func generateID() string {
	return uuid.New().String()
}

func main() {
	// ── Config from environment ──────────────────────────────────────────────
	port := getEnv("PORT", "8080")
	snsTopicARN := getEnv("SNS_TOPIC_ARN", "")
	sqsQueueURL := getEnv("SQS_QUEUE_URL", "")
	numWorkers := getEnvInt("NUM_WORKERS", 1)

	// ── AWS SDK setup ────────────────────────────────────────────────────────
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	snsClient := sns.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)

	// ── Start background SQS processor (only if queue URL is configured) ─────
	if sqsQueueURL != "" {
		processor := NewOrderProcessor(sqsClient, sqsQueueURL, numWorkers)
		go processor.Start(ctx)
		log.Printf("SQS processor started with %d workers", numWorkers)
	} else {
		log.Println("SQS_QUEUE_URL not set — running in sync-only mode (Phase 1)")
	}

	// ── HTTP routes ──────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/orders/sync", syncHandler)

	if snsTopicARN != "" {
		publisher := newSNSPublisher(snsClient, snsTopicARN)
		mux.HandleFunc("/orders/async", asyncHandler(publisher))
		log.Printf("Async endpoint enabled — SNS topic: %s", snsTopicARN)
	} else {
		log.Println("SNS_TOPIC_ARN not set — /orders/async not available (Phase 1 only)")
	}

	log.Printf("Order service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
