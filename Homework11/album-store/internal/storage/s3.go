package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Storage struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
	region   string
}

func NewS3Storage(region, bucket string) (*S3Storage, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 8 * 1024 * 1024 // 8MB parts
		u.Concurrency = 5
	})

	return &S3Storage{
		client:   client,
		uploader: uploader,
		bucket:   bucket,
		region:   region,
	}, nil
}

// Upload uploads data to S3 and returns the public URL.
func (s *S3Storage) Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 upload: %w", err)
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	return url, nil
}

// Delete removes an object from S3.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

// KeyFromIDs builds the S3 object key from album and photo IDs.
func KeyFromIDs(albumID, photoID string) string {
	return fmt.Sprintf("albums/%s/photos/%s", albumID, photoID)
}
