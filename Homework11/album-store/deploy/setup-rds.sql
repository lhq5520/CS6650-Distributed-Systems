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
