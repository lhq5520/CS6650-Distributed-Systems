package store

import (
	"context"

	"album-store/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlbumRepo struct {
	pool *pgxpool.Pool
}

func NewAlbumRepo(pool *pgxpool.Pool) *AlbumRepo {
	return &AlbumRepo{pool: pool}
}

// Upsert creates or updates an album. Returns true if created (201), false if updated (200).
func (r *AlbumRepo) Upsert(ctx context.Context, a *model.Album) (created bool, err error) {
	var result string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO albums (album_id, title, description, owner)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (album_id) DO UPDATE
		SET title = EXCLUDED.title, description = EXCLUDED.description, owner = EXCLUDED.owner
		RETURNING (xmax = 0)::text
	`, a.AlbumID, a.Title, a.Description, a.Owner).Scan(&result)
	if err != nil {
		return false, err
	}
	return result == "true", nil
}

func (r *AlbumRepo) Get(ctx context.Context, albumID string) (*model.Album, error) {
	a := &model.Album{}
	err := r.pool.QueryRow(ctx, `
		SELECT album_id, title, description, owner FROM albums WHERE album_id = $1
	`, albumID).Scan(&a.AlbumID, &a.Title, &a.Description, &a.Owner)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *AlbumRepo) List(ctx context.Context) ([]model.Album, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT album_id, title, description, owner FROM albums
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []model.Album
	for rows.Next() {
		var a model.Album
		if err := rows.Scan(&a.AlbumID, &a.Title, &a.Description, &a.Owner); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	if albums == nil {
		albums = []model.Album{}
	}
	return albums, rows.Err()
}
