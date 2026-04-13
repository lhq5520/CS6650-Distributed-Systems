package worker

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"album-store/internal/storage"
	"album-store/internal/store"
)

type PhotoJob struct {
	PhotoID string
	AlbumID string
	TmpPath string // path to temp file containing photo data
}

// PhotoCache caches photo status in memory to avoid DB queries on polling.
type PhotoCache struct {
	mu    sync.RWMutex
	items map[string]*CachedPhoto
}

type CachedPhoto struct {
	PhotoID string
	AlbumID string
	Seq     int
	Status  string
	URL     string
}

func NewPhotoCache() *PhotoCache {
	return &PhotoCache{items: make(map[string]*CachedPhoto)}
}

func (c *PhotoCache) Set(p *CachedPhoto) {
	c.mu.Lock()
	c.items[p.PhotoID] = p
	c.mu.Unlock()
}

func (c *PhotoCache) Get(photoID string) (*CachedPhoto, bool) {
	c.mu.RLock()
	p, ok := c.items[photoID]
	c.mu.RUnlock()
	return p, ok
}

func (c *PhotoCache) Delete(photoID string) {
	c.mu.Lock()
	delete(c.items, photoID)
	c.mu.Unlock()
}

type Pool struct {
	jobs      chan PhotoJob
	s3        *storage.S3Storage
	photoRepo *store.PhotoRepo
	cache     *PhotoCache
}

func NewPool(size int, s3 *storage.S3Storage, photoRepo *store.PhotoRepo, cache *PhotoCache) *Pool {
	p := &Pool{
		jobs:      make(chan PhotoJob, 1000),
		s3:        s3,
		photoRepo: photoRepo,
		cache:     cache,
	}
	for i := 0; i < size; i++ {
		go p.worker()
	}
	return p
}

func (p *Pool) Submit(job PhotoJob) {
	p.jobs <- job
}

func (p *Pool) worker() {
	for job := range p.jobs {
		p.processPhoto(job)
	}
}

func (p *Pool) processPhoto(job PhotoJob) {
	// Always clean up temp file when done
	defer os.Remove(job.TmpPath)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Open temp file for reading
	f, err := os.Open(job.TmpPath)
	if err != nil {
		log.Printf("ERROR: open temp file %s for photo %s: %v", job.TmpPath, job.PhotoID, err)
		p.photoRepo.UpdateStatus(ctx, job.PhotoID, "failed", "")
		p.updateCache(job.PhotoID, "failed", "")
		return
	}
	defer f.Close()

	key := storage.KeyFromIDs(job.AlbumID, job.PhotoID)

	url, err := p.s3.Upload(ctx, key, f, "image/jpeg")
	if err != nil {
		log.Printf("ERROR: upload photo %s: %v", job.PhotoID, err)
		p.photoRepo.UpdateStatus(ctx, job.PhotoID, "failed", "") //nolint
		p.updateCache(job.PhotoID, "failed", "")
		return
	}

	if _, err := p.photoRepo.UpdateStatus(ctx, job.PhotoID, "completed", url); err != nil {
		log.Printf("ERROR: update photo status %s: %v", job.PhotoID, err)
		return
	}

	p.updateCache(job.PhotoID, "completed", url)
}

func (p *Pool) updateCache(photoID, status, url string) {
	if cached, ok := p.cache.Get(photoID); ok {
		cached.Status = status
		cached.URL = url
		p.cache.Set(cached)
	}
}
