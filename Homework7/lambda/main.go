package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type Order struct {
	OrderID    string  `json:"order_id"`
	CustomerID int     `json:"customer_id"`
	Status     string  `json:"status"`
}

func handler(ctx context.Context, snsEvent events.SNSEvent) error {
	for _, record := range snsEvent.Records {
		var order Order
		if err := json.Unmarshal([]byte(record.SNS.Message), &order); err != nil {
			log.Printf("[LAMBDA] Failed to parse order: %v", err)
			continue
		}
		log.Printf("[LAMBDA] Processing order %s for customer %d", order.OrderID, order.CustomerID)
		time.Sleep(3 * time.Second)
		log.Printf("[LAMBDA] Completed order %s", order.OrderID)
	}
	return nil
}

func main() {
	lambda.Start(handler)
}