package handler

import (
	"encoding/json"
	"net/http"

	"album-store/internal/model"
	"album-store/internal/store"

	"github.com/go-chi/chi/v5"
)

type AlbumHandler struct {
	repo *store.AlbumRepo
}

func NewAlbumHandler(repo *store.AlbumRepo) *AlbumHandler {
	return &AlbumHandler{repo: repo}
}

func (h *AlbumHandler) Put(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")

	var a model.Album
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid json"})
		return
	}
	a.AlbumID = albumID

	created, err := h.repo.Upsert(r.Context(), &a)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "db error"})
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, a)
}

func (h *AlbumHandler) Get(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")

	a, err := h.repo.Get(r.Context(), albumID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "db error"})
		return
	}
	if a == nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *AlbumHandler) List(w http.ResponseWriter, r *http.Request) {
	albums, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "db error"})
		return
	}
	writeJSON(w, http.StatusOK, albums)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
