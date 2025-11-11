package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLiteStorage implements Storage using a SQLite database.
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage opens the SQLite database at the provided DSN.
func NewSQLiteStorage(dsn string) (*SQLiteStorage, error) {
	// TODO: Register a SQLite driver (e.g., modernc.org/sqlite or mattn/go-sqlite3) before calling this in production.
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	storage := &SQLiteStorage{db: db}
	if err := storage.initSchema(); err != nil {
		return nil, err
	}
	return storage, nil
}

func (s *SQLiteStorage) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS clipboard_items (
local_id INTEGER PRIMARY KEY AUTOINCREMENT,
remote_id TEXT,
user_id TEXT NOT NULL,
device_id TEXT NOT NULL,
content_type TEXT NOT NULL,
content TEXT NOT NULL,
created_at TIMESTAMP NOT NULL,
updated_at TIMESTAMP NOT NULL,
synced_at TIMESTAMP,
is_deleted INTEGER NOT NULL DEFAULT 0,
favorite INTEGER NOT NULL DEFAULT 0
);`,
		`CREATE INDEX IF NOT EXISTS idx_clipboard_items_synced ON clipboard_items(remote_id, synced_at);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_clipboard_items_remote_id ON clipboard_items(remote_id);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the database resources.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// SaveNewItem inserts a new clipboard item into the local database.
func (s *SQLiteStorage) SaveNewItem(ctx context.Context, item *LocalClipboardItem) error {
	if item == nil {
		return fmt.Errorf("item is nil")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO clipboard_items (
user_id, device_id, content_type, content, created_at, updated_at, favorite, is_deleted
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		item.UserID, item.DeviceID, item.ContentType, item.Content, item.CreatedAt, item.UpdatedAt, boolToInt(item.Favorite), boolToInt(item.IsDeleted))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	item.LocalID = id
	return nil
}

// ListItems returns clipboard items based on the provided filter.
func (s *SQLiteStorage) ListItems(ctx context.Context, filter ListFilter) ([]LocalClipboardItem, error) {
	queryBuilder := strings.Builder{}
	args := make([]any, 0)
	queryBuilder.WriteString(`SELECT local_id, remote_id, user_id, device_id, content_type, content, created_at, updated_at, synced_at, is_deleted, favorite FROM clipboard_items`)
	conditions := make([]string, 0)
	if !filter.IncludeDeleted {
		conditions = append(conditions, "is_deleted = 0")
	}
	if filter.Query != "" {
		conditions = append(conditions, "content LIKE ?")
		args = append(args, "%"+filter.Query+"%")
	}
	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}
	queryBuilder.WriteString(" ORDER BY created_at DESC LIMIT ? OFFSET ?")
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LocalClipboardItem, 0)
	for rows.Next() {
		var item LocalClipboardItem
		if err := rows.Scan(&item.LocalID, &item.RemoteID, &item.UserID, &item.DeviceID, &item.ContentType, &item.Content, &item.CreatedAt, &item.UpdatedAt, &item.SyncedAt, &item.IsDeleted, &item.Favorite); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListUnsyncedItems returns unsynced clipboard items up to the provided limit.
func (s *SQLiteStorage) ListUnsyncedItems(ctx context.Context, limit int) ([]LocalClipboardItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT local_id, remote_id, user_id, device_id, content_type, content, created_at, updated_at, synced_at, is_deleted, favorite FROM clipboard_items WHERE (synced_at IS NULL OR updated_at > synced_at) LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LocalClipboardItem, 0)
	for rows.Next() {
		var item LocalClipboardItem
		if err := rows.Scan(&item.LocalID, &item.RemoteID, &item.UserID, &item.DeviceID, &item.ContentType, &item.Content, &item.CreatedAt, &item.UpdatedAt, &item.SyncedAt, &item.IsDeleted, &item.Favorite); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// MarkAsSynced updates the synchronization metadata for an item.
func (s *SQLiteStorage) MarkAsSynced(ctx context.Context, localID int64, remoteID string, syncedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE clipboard_items SET remote_id = ?, synced_at = ?, updated_at = ?, is_deleted = is_deleted WHERE local_id = ?`, remoteID, syncedAt, syncedAt, localID)
	return err
}

// SoftDelete marks the item as deleted without removing it from storage.
func (s *SQLiteStorage) SoftDelete(ctx context.Context, localID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE clipboard_items SET is_deleted = 1, updated_at = ? WHERE local_id = ?`, time.Now(), localID)
	return err
}

// SetFavorite toggles the favorite state of a clipboard item.
func (s *SQLiteStorage) SetFavorite(ctx context.Context, localID int64, favorite bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE clipboard_items SET favorite = ?, updated_at = ? WHERE local_id = ?`, boolToInt(favorite), time.Now(), localID)
	return err
}

// UpsertFromRemote merges remote clipboard changes into the local store.
func (s *SQLiteStorage) UpsertFromRemote(ctx context.Context, item RemoteClipboardItem) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO clipboard_items (
remote_id, user_id, device_id, content_type, content, created_at, updated_at, favorite, is_deleted, synced_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(remote_id) DO UPDATE SET
content = excluded.content,
content_type = excluded.content_type,
favorite = excluded.favorite,
is_deleted = excluded.is_deleted,
updated_at = excluded.updated_at,
synced_at = excluded.updated_at
`, item.RemoteID, item.UserID, item.DeviceID, item.ContentType, item.Content, item.CreatedAt, item.UpdatedAt, boolToInt(item.Favorite), boolToInt(item.Deleted), item.UpdatedAt)
	return err
}

// GetItem retrieves a single clipboard item by local identifier.
func (s *SQLiteStorage) GetItem(ctx context.Context, localID int64) (*LocalClipboardItem, error) {
	row := s.db.QueryRowContext(ctx, `SELECT local_id, remote_id, user_id, device_id, content_type, content, created_at, updated_at, synced_at, is_deleted, favorite FROM clipboard_items WHERE local_id = ?`, localID)
	var item LocalClipboardItem
	if err := row.Scan(&item.LocalID, &item.RemoteID, &item.UserID, &item.DeviceID, &item.ContentType, &item.Content, &item.CreatedAt, &item.UpdatedAt, &item.SyncedAt, &item.IsDeleted, &item.Favorite); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
