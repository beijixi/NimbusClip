package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LocalClipboardItem represents the clipboard data stored on the agent.
type LocalClipboardItem struct {
	LocalID     int64
	RemoteID    sql.NullString
	UserID      string
	DeviceID    string
	ContentType string
	Content     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SyncedAt    sql.NullTime
	IsDeleted   bool
	Favorite    bool
}

// Storage defines the persistence contract for clipboard data on the agent.
type Storage interface {
	SaveNewItem(ctx context.Context, item *LocalClipboardItem) error
	ListItems(ctx context.Context, filter ListFilter) ([]LocalClipboardItem, error)
	ListUnsyncedItems(ctx context.Context, limit int) ([]LocalClipboardItem, error)
	MarkAsSynced(ctx context.Context, localID int64, remoteID string, syncedAt time.Time) error
	SoftDelete(ctx context.Context, localID int64) error
	SetFavorite(ctx context.Context, localID int64, favorite bool) error
	UpsertFromRemote(ctx context.Context, item RemoteClipboardItem) error
	GetItem(ctx context.Context, localID int64) (*LocalClipboardItem, error)
}

// ListFilter defines filters for querying clipboard items.
type ListFilter struct {
	Query          string
	Page           int
	PageSize       int
	IncludeDeleted bool
}

// RemoteClipboardItem describes a clipboard record from the server that should be merged locally.
type RemoteClipboardItem struct {
	RemoteID    string
	UserID      string
	DeviceID    string
	ContentType string
	Content     string
	Favorite    bool
	Deleted     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ErrItemNotFound indicates that a requested item was not located.
var ErrItemNotFound = errors.New("clipboard item not found")
