package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func main() {
	// Validate arguments
	// Example: reducer result1.json result2.json result3.json final.json
	if len(os.Args) < 3 {
		fmt.Println("Usage: reducer <result1-url> <result2-url> ... <output-url>")
		fmt.Println("Example: reducer s3://bucket/result1.json s3://bucket/result2.json s3://bucket/final.json")
		os.Exit(1)
	}

	// The last argument is the output URL
	outputURL := os.Args[len(os.Args)-1]

	// All previous arguments are input URLs
	inputURLs := os.Args[1 : len(os.Args)-1]

	log.Printf("Input files: %d", len(inputURLs))
	log.Printf("Output: %s", outputURL)

	// Accumulated word counts
	finalCount := make(map[string]int)

	// Download and merge each result
	for i, url := range inputURLs {
		log.Printf("Processing %d: %s", i+1, url)

		// Download JSON
		jsonStr, err := downloadFromS3(url)
		if err != nil {
			log.Fatalf("Error downloading %s: %v", url, err)
		}

		// Parse JSON
		var count map[string]int
		err = json.Unmarshal([]byte(jsonStr), &count)
		if err != nil {
			log.Fatalf("Error parsing JSON: %v", err)
		}

		log.Printf("  Words in this file: %d", len(count))

		// Merge into finalCount
		for word, cnt := range count {
			finalCount[word] += cnt
		}
	}

	log.Printf("Total unique words: %d", len(finalCount))
	log.Printf("Final count: %v", finalCount)

	// Convert to JSON
	finalJSON, err := json.Marshal(finalCount)
	if err != nil {
		log.Fatalf("Error marshaling final JSON: %v", err)
	}

	// Upload final result
	err = uploadToS3(outputURL, string(finalJSON))
	if err != nil {
		log.Fatalf("Error uploading: %v", err)
	}

	log.Printf("✓ Success! Final result saved to %s", outputURL)
}

// --- S3 download ---
func downloadFromS3(s3URL string) (string, error) {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	}))

	downloader := s3manager.NewDownloader(sess)

	s3URL = strings.TrimPrefix(s3URL, "s3://")
	parts := strings.SplitN(s3URL, "/", 2)

	bucket := parts[0]
	key := parts[1]

	buff := &aws.WriteAtBuffer{}

	_, err := downloader.Download(buff, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return "", err
	}

	return string(buff.Bytes()), nil
}

// --- S3 upload ---
func uploadToS3(s3URL, content string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	}))

	uploader := s3manager.NewUploader(sess)

	s3URL = strings.TrimPrefix(s3URL, "s3://")
	parts := strings.SplitN(s3URL, "/", 2)

	bucket := parts[0]
	key := parts[1]

	log.Printf("Uploading: bucket=%s, key=%s", bucket, key)

	reader := strings.NewReader(content)

	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   reader,
	})

	return err
}
