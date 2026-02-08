package main

import (
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
	// Validate command-line arguments
	if len(os.Args) != 3 {
		fmt.Println("Usage: splitter <input-s3-url> <output-s3-prefix>")
		fmt.Println("Example: splitter s3://bucket/input.txt s3://bucket/chunks/")
		os.Exit(1)
	}

	inputURL := os.Args[1]
	outputPrefix := os.Args[2]

	log.Printf("Input: %s", inputURL)
	log.Printf("Output prefix: %s", outputPrefix)

	// Step 1: Download the file
	text, err := downloadFromS3(inputURL)
	if err != nil {
		log.Fatalf("Error downloading: %v", err)
	}

	log.Printf("Downloaded %d bytes", len(text))

	// Step 2: Split into lines
	lines := strings.Split(text, "\n")
	totalLines := len(lines)

	log.Printf("Total lines: %d", totalLines)

	// Step 3: Calculate lines per chunk
	chunkSize := totalLines / 3
	remainder := totalLines % 3

	// Step 4: Create 3 chunks
	chunks := make([]string, 3)
	start := 0

	for i := 0; i < 3; i++ {
		end := start + chunkSize
		if i < remainder {
			end++ // Distribute remainder to early chunks
		}

		chunks[i] = strings.Join(lines[start:end], "\n")
		chunkLines := end - start
		start = end

		log.Printf("Chunk %d: %d lines", i+1, chunkLines)
	}

	// Step 5: Upload 3 chunks
	outputPrefix = strings.TrimSuffix(outputPrefix, "/")

	for i := 0; i < 3; i++ {
		chunkURL := fmt.Sprintf("%s/chunk%d.txt", outputPrefix, i+1)

		err := uploadToS3(chunkURL, chunks[i])
		if err != nil {
			log.Fatalf("Error uploading chunk %d: %v", i+1, err)
		}

		log.Printf("✓ Uploaded: %s", chunkURL)
	}

	log.Printf("✓ Success! 3 chunks created")
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

	log.Printf("Downloading: bucket=%s, key=%s", bucket, key)

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
