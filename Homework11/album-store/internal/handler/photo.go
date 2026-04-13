package handler

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"

	"album-store/internal/model"
	"album-store/internal/storage"
	"album-store/internal/store"
	"album-store/internal/worker"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Limit concurrent S3 uploads to control memory/GC pressure
var uploadSem = make(chan struct{}, 20)

type PhotoHandler struct {
	photoRepo *store.PhotoRepo
	s3        *storage.S3Storage
	cache     *worker.PhotoCache
}

func NewPhotoHandler(photoRepo *store.PhotoRepo, s3 *storage.S3Storage, cache *worker.PhotoCache) *PhotoHandler {
	return &PhotoHandler{
		photoRepo: photoRepo,
		s3:        s3,
		cache:     cache,
	}
}

func (h *PhotoHandler) Upload(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")

	reader, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid multipart form"})
		return
	}

	// Find the "photo" part
	var data []byte
	for {
		p, err := reader.NextPart()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "missing photo field"})
			return
		}
		if p.FormName() == "photo" {
			data, err = io.ReadAll(p)
			p.Close()
			if err != nil {
				log.Printf("ERROR: read photo: %v", err)
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "failed to read photo"})
				return
			}
			break
		}
		p.Close()
	}

	photoID := uuid.New().String()

	seq, err := h.photoRepo.AllocateSeqAndInsert(r.Context(), photoID, albumID)
	if err != nil {
		log.Printf("ERROR: allocate seq for photo %s in album %s: %v", photoID, albumID, err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "db error"})
		return
	}

	h.cache.Set(&worker.CachedPhoto{
		PhotoID: photoID,
		AlbumID: albumID,
		Seq:     seq,
		Status:  "processing",
	})

	// Launch goroutine to upload to S3
	go h.processUpload(photoID, albumID, seq, data)

	writeJSON(w, http.StatusAccepted, model.PhotoAccepted{
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	})
}

func (h *PhotoHandler) processUpload(photoID, albumID string, seq int, data []byte) {
	// Acquire semaphore to limit concurrent uploads
	uploadSem <- struct{}{}
	defer func() { <-uploadSem }()

	ctx := context.Background()
	key := storage.KeyFromIDs(albumID, photoID)

	url, err := h.s3.Upload(ctx, key, bytes.NewReader(data), "image/jpeg")
	if err != nil {
		log.Printf("ERROR: upload photo %s to S3: %v", photoID, err)
		updated, _ := h.photoRepo.UpdateStatus(ctx, photoID, "failed", "")
		if updated {
			h.cache.Set(&worker.CachedPhoto{PhotoID: photoID, AlbumID: albumID, Seq: seq, Status: "failed"})
		}
		return
	}

	updated, err := h.photoRepo.UpdateStatus(ctx, photoID, "completed", url)
	if err != nil {
		log.Printf("ERROR: update photo status %s: %v", photoID, err)
		return
	}
	if updated {
		h.cache.Set(&worker.CachedPhoto{
			PhotoID: photoID, AlbumID: albumID, Seq: seq,
			Status: "completed", URL: url,
		})
	}
}

func (h *PhotoHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")

	if cached, ok := h.cache.Get(photoID); ok && cached.AlbumID == albumID {
		writeJSON(w, http.StatusOK, model.Photo{
			PhotoID: cached.PhotoID,
			AlbumID: cached.AlbumID,
			Seq:     cached.Seq,
			Status:  cached.Status,
			URL:     cached.URL,
		})
		return
	}

	p, err := h.photoRepo.Get(r.Context(), albumID, photoID)
	if err != nil {
		log.Printf("ERROR: get photo %s: %v", photoID, err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "db error"})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *PhotoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")

	url, err := h.photoRepo.Delete(r.Context(), albumID, photoID)
	if err != nil {
		log.Printf("ERROR: delete photo %s: %v", photoID, err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "db error"})
		return
	}

	h.cache.Delete(photoID)

	if url != "" {
		key := storage.KeyFromIDs(albumID, photoID)
		go h.s3.Delete(context.Background(), key)
	}

	w.WriteHeader(http.StatusNoContent)
}
