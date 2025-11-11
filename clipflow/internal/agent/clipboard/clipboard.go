package clipboard

import (
	"context"

	"clipflow/internal/common/model"
)

// ChangeHandler is invoked when the clipboard contents change.
type ChangeHandler func(ctx context.Context, item *model.ClipboardItem) error

// ClipboardAdapter defines platform-specific clipboard operations.
type ClipboardAdapter interface {
	// ReadContent returns the current clipboard content snapshot.
	ReadContent(ctx context.Context) (*model.ClipboardItem, error)
	// Watch subscribes to clipboard changes until the context is cancelled.
	Watch(ctx context.Context, handler ChangeHandler) error
	// WriteContent replaces the system clipboard content with the given item.
	WriteContent(ctx context.Context, item *model.ClipboardItem) error
}
