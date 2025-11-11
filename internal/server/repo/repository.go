package repo

import (
	"context"

	"clipflow/internal/common/model"
)

// ClipboardRepository defines data access operations for clipboard items.
type ClipboardRepository interface {
	CreateOrUpdateClipboardItem(ctx context.Context, item model.ClipboardItem) (model.ClipboardItem, error)
	ListClipboardItemsForUser(ctx context.Context, userID string, limit int) ([]model.ClipboardItem, error)
	ListChangesSinceVersion(ctx context.Context, userID string, lastVersion int64) ([]model.ClipboardItem, error)
}
