package model

import "time"

// ClipboardItem represents a clipboard record stored on the synchronization service.
type ClipboardItem struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	DeviceID    string     `json:"deviceId,omitempty"`
	ContentType string     `json:"contentType"`
	Content     string     `json:"content"`
	Favorite    bool       `json:"favorite"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
	Version     int64      `json:"version"`
}

// IsDeleted returns whether the clipboard item is marked as deleted.
func (c ClipboardItem) IsDeleted() bool {
	return c.DeletedAt != nil
}
