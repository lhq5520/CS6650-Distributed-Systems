package model

type Album struct {
	AlbumID     string `json:"album_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

type Photo struct {
	PhotoID string `json:"photo_id"`
	AlbumID string `json:"album_id"`
	Seq     int    `json:"seq"`
	Status  string `json:"status"`
	URL     string `json:"url,omitempty"`
}

type PhotoAccepted struct {
	PhotoID string `json:"photo_id"`
	Seq     int    `json:"seq"`
	Status  string `json:"status"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
