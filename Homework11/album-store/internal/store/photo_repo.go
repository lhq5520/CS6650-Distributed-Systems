package store

import (
	"context"

	"album-store/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PhotoRepo struct {
	pool *pgxpool.Pool
}

func NewPhotoRepo(pool *pgxpool.Pool) *PhotoRepo {
	return &PhotoRepo{pool: pool}
}

// AllocateSeqAndInsert atomically increments the album's seq counter and inserts a new photo row.
// Returns the assigned seq number.
func (r *PhotoRepo) AllocateSeqAndInsert(ctx context.Context, photoID, albumID string) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var seq int
	err = tx.QueryRow(ctx, `
		UPDATE albums SET next_seq = next_seq + 1 WHERE album_id = $1 RETURNING next_seq
	`, albumID).Scan(&seq)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO photos (photo_id, album_id, seq, status) VALUES ($1, $2, $3, 'processing')
	`, photoID, albumID, seq)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return seq, nil
}

func (r *PhotoRepo) Get(ctx context.Context, albumID, photoID string) (*model.Photo, error) {
	p := &model.Photo{}
	var url *string
	err := r.pool.QueryRow(ctx, `
		SELECT photo_id, album_id, seq, status, url FROM photos WHERE photo_id = $1 AND album_id = $2
	`, photoID, albumID).Scan(&p.PhotoID, &p.AlbumID, &p.Seq, &p.Status, &url)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if url != nil {
		p.URL = *url
	}
	return p, nil
}

// UpdateStatus updates photo status only if it's still 'processing'.
// Returns true if the row was actually updated.
func (r *PhotoRepo) UpdateStatus(ctx context.Context, photoID, status, url string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE photos SET status = $1, url = $2 WHERE photo_id = $3 AND status = 'processing'
	`, status, url, photoID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PhotoRepo) Delete(ctx context.Context, albumID, photoID string) (url string, err error) {
	var u *string
	err = r.pool.QueryRow(ctx, `
		DELETE FROM photos WHERE photo_id = $1 AND album_id = $2 RETURNING url
	`, photoID, albumID).Scan(&u)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if u != nil {
		return *u, nil
	}
	return "", nil
}
