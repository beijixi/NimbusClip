package repo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"clipflow/internal/common/model"
)

// SQLClipboardRepository implements ClipboardRepository using a relational database.
type SQLClipboardRepository struct {
	db *sql.DB
}

// NewSQLClipboardRepository creates a new repository backed by the provided database connection.
func NewSQLClipboardRepository(db *sql.DB) (*SQLClipboardRepository, error) {
	repo := &SQLClipboardRepository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *SQLClipboardRepository) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS clipboard_items (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL,
device_id TEXT,
content_type TEXT NOT NULL,
content TEXT NOT NULL,
favorite INTEGER NOT NULL DEFAULT 0,
deleted_at TIMESTAMP,
created_at TIMESTAMP NOT NULL,
updated_at TIMESTAMP NOT NULL,
version INTEGER NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_version ON clipboard_items(user_id, version);`,
		`CREATE TABLE IF NOT EXISTS sync_state (
id INTEGER PRIMARY KEY CHECK (id = 1),
last_version INTEGER NOT NULL
);`,
		`INSERT INTO sync_state(id, last_version)
SELECT 1, 0 WHERE NOT EXISTS (SELECT 1 FROM sync_state WHERE id = 1);`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// CreateOrUpdateClipboardItem inserts or updates a clipboard item, returning the persisted value including version.
func (r *SQLClipboardRepository) CreateOrUpdateClipboardItem(ctx context.Context, item model.ClipboardItem) (model.ClipboardItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ClipboardItem{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if item.ID == "" {
		item.ID = generateID()
	}

	existing, err := r.loadItem(ctx, tx, item.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.ClipboardItem{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		existing = nil
	}

	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}

	version, err := r.nextVersion(ctx, tx)
	if err != nil {
		return model.ClipboardItem{}, err
	}
	item.Version = version

	if item.DeletedAt != nil && item.DeletedAt.IsZero() {
		item.DeletedAt = nil
	}

	if existing == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO clipboard_items (id, user_id, device_id, content_type, content, favorite, deleted_at, created_at, updated_at, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.UserID, item.DeviceID, item.ContentType, item.Content, boolToInt(item.Favorite), item.DeletedAt, item.CreatedAt, item.UpdatedAt, item.Version)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE clipboard_items SET user_id = ?, device_id = ?, content_type = ?, content = ?, favorite = ?, deleted_at = ?, created_at = ?, updated_at = ?, version = ? WHERE id = ?`,
			item.UserID, item.DeviceID, item.ContentType, item.Content, boolToInt(item.Favorite), item.DeletedAt, existing.CreatedAt, item.UpdatedAt, item.Version, item.ID)
		item.CreatedAt = existing.CreatedAt
	}
	if err != nil {
		return model.ClipboardItem{}, err
	}

	if err := tx.Commit(); err != nil {
		return model.ClipboardItem{}, err
	}
	return item, nil
}

// ListClipboardItemsForUser retrieves clipboard items for a user limited by the specified count.
func (r *SQLClipboardRepository) ListClipboardItemsForUser(ctx context.Context, userID string, limit int) ([]model.ClipboardItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, device_id, content_type, content, favorite, deleted_at, created_at, updated_at, version FROM clipboard_items WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.ClipboardItem, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListChangesSinceVersion returns clipboard items with version higher than the specified value.
func (r *SQLClipboardRepository) ListChangesSinceVersion(ctx context.Context, userID string, lastVersion int64) ([]model.ClipboardItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, device_id, content_type, content, favorite, deleted_at, created_at, updated_at, version FROM clipboard_items WHERE user_id = ? AND version > ? ORDER BY version ASC`, userID, lastVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.ClipboardItem, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLClipboardRepository) loadItem(ctx context.Context, tx *sql.Tx, id string) (*model.ClipboardItem, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, user_id, device_id, content_type, content, favorite, deleted_at, created_at, updated_at, version FROM clipboard_items WHERE id = ?`, id)
	item, err := scanItem(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *SQLClipboardRepository) nextVersion(ctx context.Context, tx *sql.Tx) (int64, error) {
	row := tx.QueryRowContext(ctx, `UPDATE sync_state SET last_version = last_version + 1 WHERE id = 1 RETURNING last_version`)
	var version int64
	if err := row.Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func scanItem(scanner interface{ Scan(dest ...any) error }) (model.ClipboardItem, error) {
	var (
		item        model.ClipboardItem
		favoriteInt int
		deletedAt   sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &item.UserID, &item.DeviceID, &item.ContentType, &item.Content, &favoriteInt, &deletedAt, &item.CreatedAt, &item.UpdatedAt, &item.Version); err != nil {
		return model.ClipboardItem{}, err
	}
	item.Favorite = favoriteInt == 1
	if deletedAt.Valid {
		t := deletedAt.Time
		item.DeletedAt = &t
	}
	return item, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func generateID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
