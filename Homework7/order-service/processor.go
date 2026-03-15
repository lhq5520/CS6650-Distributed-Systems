package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// ─── SNS Publisher ────────────────────────────────────────────────────────────

type SNSPublisher interface {
	Publish(ctx context.Context, order Order) error
}

type snsPublisher struct {
	client   *sns.Client
	topicARN string
}

func newSNSPublisher(client *sns.Client, topicARN string) SNSPublisher {
	return &snsPublisher{client: client, topicARN: topicARN}
}

func (p *snsPublisher) Publish(ctx context.Context, order Order) error {
	body, err := json.Marshal(order)
	if err != nil {
		return err
	}
	_, err = p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.topicARN),
		Message:  aws.String(string(body)),
	})
	return err
}

// ─── SQS Processor ────────────────────────────────────────────────────────────

type OrderProcessor struct {
	sqsClient  *sqs.Client
	queueURL   string
	numWorkers int
}

func NewOrderProcessor(sqsClient *sqs.Client, queueURL string, numWorkers int) *OrderProcessor {
	return &OrderProcessor{
		sqsClient:  sqsClient,
		queueURL:   queueURL,
		numWorkers: numWorkers,
	}
}

// Start launches numWorkers goroutines, each continuously polling SQS.
func (p *OrderProcessor) Start(ctx context.Context) {
	log.Printf("[PROCESSOR] Starting %d worker goroutines", p.numWorkers)
	var wg sync.WaitGroup
	for i := 0; i < p.numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.runWorker(ctx, workerID)
		}(i)
	}
	wg.Wait()
}

// runWorker continuously polls SQS and processes messages.
func (p *OrderProcessor) runWorker(ctx context.Context, workerID int) {
	log.Printf("[WORKER-%d] Started", workerID)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[WORKER-%d] Shutting down", workerID)
			return
		default:
		}

		// Long-poll: wait up to 20s for messages, receive up to 10 at once
		resp, err := p.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(p.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[WORKER-%d] ReceiveMessage error: %v", workerID, err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Process each message concurrently within this worker
		var wg sync.WaitGroup
		for _, msg := range resp.Messages {
			wg.Add(1)
			go func(m sqstypes.Message) {
				defer wg.Done()
				p.processMessage(ctx, workerID, m)
			}(msg)
		}
		wg.Wait()
	}
}

// processMessage handles a single SQS message.
func (p *OrderProcessor) processMessage(ctx context.Context, workerID int, msg sqstypes.Message) {
	// SNS wraps the message body — unwrap it
	var snsEnvelope struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal([]byte(*msg.Body), &snsEnvelope); err != nil {
		log.Printf("[WORKER-%d] Failed to parse SNS envelope: %v", workerID, err)
		return
	}

	var order Order
	if err := json.Unmarshal([]byte(snsEnvelope.Message), &order); err != nil {
		log.Printf("[WORKER-%d] Failed to parse order: %v", workerID, err)
		return
	}

	log.Printf("[WORKER-%d] Processing order %s for customer %d", workerID, order.OrderID, order.CustomerID)

	// Simulate 3-second payment processing (same bottleneck as sync)
	done := make(chan struct{}, 1)
	go func() {
		time.Sleep(3 * time.Second)
		done <- struct{}{}
	}()
	<-done

	log.Printf("[WORKER-%d] Completed order %s", workerID, order.OrderID)

	// Delete message from SQS after successful processing
	_, err := p.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		log.Printf("[WORKER-%d] Failed to delete message: %v", workerID, err)
	}
}
