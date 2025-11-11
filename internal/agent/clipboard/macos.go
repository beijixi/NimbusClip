//go:build darwin

package clipboard

import (
	"context"
	"errors"

	"clipflow/internal/common/model"
)

// MacOSAdapter provides a placeholder macOS clipboard implementation.
type MacOSAdapter struct{}

// NewMacOSAdapter creates a new macOS clipboard adapter.
func NewMacOSAdapter() *MacOSAdapter {
	return &MacOSAdapter{}
}

// ReadContent returns the current clipboard contents.
func (m *MacOSAdapter) ReadContent(ctx context.Context) (*model.ClipboardItem, error) {
	// TODO: Implement macOS clipboard read via NSPasteboard APIs.
	return nil, errors.New("clipboard read not implemented")
}

// Watch monitors the clipboard for changes.
func (m *MacOSAdapter) Watch(ctx context.Context, handler ChangeHandler) error {
	// TODO: Implement macOS clipboard watcher using polling or event taps.
	return errors.New("clipboard watch not implemented")
}

// WriteContent writes the provided item to the system clipboard.
func (m *MacOSAdapter) WriteContent(ctx context.Context, item *model.ClipboardItem) error {
	// TODO: Implement macOS clipboard write via NSPasteboard APIs.
	return errors.New("clipboard write not implemented")
}
