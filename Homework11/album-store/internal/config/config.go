package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	S3Bucket    string
	AWSRegion   string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/albumstore?sslmode=disable"),
		S3Bucket:    getEnv("S3_BUCKET", "album-store-photos"),
		AWSRegion:   getEnv("AWS_REGION", "us-west-2"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
