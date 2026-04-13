package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"album-store/internal/config"
	"album-store/internal/handler"
	"album-store/internal/storage"
	"album-store/internal/store"
	"album-store/internal/worker"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	// Database
	pool, err := store.NewPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run schema migration
	if err := runMigration(pool); err != nil {
		log.Fatalf("Failed to run migration: %v", err)
	}

	// Repositories
	albumRepo := store.NewAlbumRepo(pool)
	photoRepo := store.NewPhotoRepo(pool)

	// S3
	s3Storage, err := storage.NewS3Storage(cfg.AWSRegion, cfg.S3Bucket)
	if err != nil {
		log.Fatalf("Failed to init S3: %v", err)
	}

	// Photo cache for fast status polling
	photoCache := worker.NewPhotoCache()

	// Handlers
	albumHandler := handler.NewAlbumHandler(albumRepo)
	photoHandler := handler.NewPhotoHandler(photoRepo, s3Storage, photoCache)

	// Router
	r := chi.NewRouter()

	r.Get("/health", handler.HealthCheck)

	r.Put("/albums/{album_id}", albumHandler.Put)
	r.Get("/albums/{album_id}", albumHandler.Get)
	r.Get("/albums", albumHandler.List)

	r.Post("/albums/{album_id}/photos", photoHandler.Upload)
	r.Get("/albums/{album_id}/photos/{photo_id}", photoHandler.GetStatus)
	r.Delete("/albums/{album_id}/photos/{photo_id}", photoHandler.Delete)

	// Server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  0, // no read timeout — large uploads stream after 202
		WriteTimeout: 0, // no write timeout — handler continues after response flush
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("Server starting on port %s", cfg.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func runMigration(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schema := `
	CREATE TABLE IF NOT EXISTS albums (
		album_id    UUID PRIMARY KEY,
		title       TEXT NOT NULL,
		description TEXT NOT NULL,
		owner       TEXT NOT NULL,
		next_seq    INT NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS photos (
		photo_id   UUID PRIMARY KEY,
		album_id   UUID NOT NULL REFERENCES albums(album_id),
		seq        INT NOT NULL,
		status     TEXT NOT NULL DEFAULT 'processing',
		url        TEXT,
		created_at TIMESTAMPTZ DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS idx_photos_album ON photos(album_id);
	CREATE INDEX IF NOT EXISTS idx_photos_album_photo ON photos(album_id, photo_id);
	`
	_, err := pool.Exec(ctx, schema)
	return err
}
