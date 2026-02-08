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
	// Check command-line arguments
	if len(os.Args) != 3 {
		fmt.Println("Usage: mapper <input-s3-url> <output-s3-url>")
		fmt.Println("Example: mapper s3://bucket/chunk1.txt s3://bucket/result1.json")
		os.Exit(1)
	}

	inputURL := os.Args[1]
	outputURL := os.Args[2]

	log.Printf("Input: %s", inputURL)
	log.Printf("Output: %s", outputURL)

	// TODO 1: Download the file from S3
	text, err := downloadFromS3(inputURL)
	if err != nil {
		log.Fatalf("Error downloading: %v", err)
	}

	log.Printf("Downloaded %d bytes", len(text))

	// TODO 2: Count word frequencies
	wordCount := countWords(text)

	log.Printf("Word count: %v", wordCount)

	// TODO 3: Convert to JSON
	jsonData, err := json.Marshal(wordCount)
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	// TODO 4: Upload to S3
	err = uploadToS3(outputURL, string(jsonData))
	if err != nil {
		log.Fatalf("Error uploading: %v", err)
	}

	log.Printf("✓ Success! Result saved to %s", outputURL)
}

// ========================================
// TODO 1: Implement S3 download
// ========================================
func downloadFromS3(s3URL string) (string, error) {
	// Create AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	}))

	downloader := s3manager.NewDownloader(sess)

	// TODO: Parse S3 URL
	// Input: "s3://bucket-name/path/file.txt"
	// Need: bucket = "bucket-name", key = "path/file.txt"

	s3URL = strings.TrimPrefix(s3URL, "s3://")
	parts := strings.SplitN(s3URL, "/", 2)

	bucket := parts[0]
	key := parts[1]

	log.Printf("Downloading: bucket=%s, key=%s", bucket, key)

	// TODO: Download to memory
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

// ========================================
// TODO 2: Implement word frequency count
// ========================================
func countWords(text string) map[string]int {
	wordCount := make(map[string]int)

	// TODO: Implement this
	// Hints:
	// 1. To lower case
	// 2. Split words
	// 3. Trim punctuation
	// 4. Count

	// Step 1: To lower case
	text = strings.ToLower(text)

	// Step 2: Split words
	words := strings.Fields(text)

	// Step 3 & 4: Trim punctuation and count
	for _, word := range words {
		// Trim punctuation
		word = strings.Trim(word, ".,!?;:\"'()[]")

		if word != "" {
			wordCount[word]++
		}
	}

	return wordCount
}

// ========================================
// TODO 3: Implement S3 upload
// ========================================
func uploadToS3(s3URL, content string) error {
	// Create AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	}))

	uploader := s3manager.NewUploader(sess)

	// TODO: Parse S3 URL
	s3URL = strings.TrimPrefix(s3URL, "s3://")
	parts := strings.SplitN(s3URL, "/", 2)

	bucket := parts[0]
	key := parts[1]

	log.Printf("Uploading: bucket=%s, key=%s", bucket, key)

	// TODO: Upload
	reader := strings.NewReader(content)

	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   reader,
	})

	return err
}
