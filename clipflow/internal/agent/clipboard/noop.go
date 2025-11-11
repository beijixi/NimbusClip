//go:build !darwin

package clipboard

import (
	"context"
	"errors"

	"clipflow/internal/common/model"
)

// NewMacOSAdapter returns a stub implementation when building for unsupported platforms.
func NewMacOSAdapter() ClipboardAdapter {
	return &unsupportedAdapter{}
}

type unsupportedAdapter struct{}

func (u *unsupportedAdapter) ReadContent(ctx context.Context) (*model.ClipboardItem, error) {
	return nil, errors.New("clipboard access not supported on this platform")
}

func (u *unsupportedAdapter) Watch(ctx context.Context, handler ChangeHandler) error {
	return errors.New("clipboard watch not supported on this platform")
}

func (u *unsupportedAdapter) WriteContent(ctx context.Context, item *model.ClipboardItem) error {
	return errors.New("clipboard write not supported on this platform")
}
