package documents

import (
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Filename    string    `json:"filename"`
	StoragePath string    `json:"-"`
	Version     int       `json:"version"`
	IsTemplate  bool      `json:"is_template"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Version struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}
