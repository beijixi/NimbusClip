//go:build !darwin && !windows

package hotkey

import "errors"

type stubPlatform struct{}

func newPlatform() (platform, error) {
	return &stubPlatform{}, nil
}

func (s *stubPlatform) register(combo combination, callback func()) (func(), error) {
	return func() {}, errors.New("global hotkeys are not supported on this platform yet")
}
