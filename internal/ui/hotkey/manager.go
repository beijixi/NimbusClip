package hotkey

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type combination struct {
	Key   string
	Ctrl  bool
	Alt   bool
	Shift bool
	Super bool
}

// Manager coordinates OS-level hotkey registration.
type Manager struct {
	mu       sync.Mutex
	combo    combination
	cancel   func()
	platform platform
}

type platform interface {
	register(combo combination, callback func()) (func(), error)
}

// NewManager constructs a new hotkey manager for the current platform.
func NewManager() (*Manager, error) {
	plat, err := newPlatform()
	if err != nil {
		return nil, err
	}
	return &Manager{platform: plat}, nil
}

// Set registers the provided combination and callback, replacing any existing hotkey.
func (m *Manager) Set(combo string, cb func()) error {
	if cb == nil {
		return errors.New("callback must not be nil")
	}
	parsed, err := parseCombination(combo)
	if err != nil {
		return err
	}
	cancel, err := m.platform.register(parsed, cb)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.combo = parsed
	m.cancel = cancel
	m.mu.Unlock()
	return nil
}

// Current returns the active hotkey string representation.
func (m *Manager) Current() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return formatCombination(m.combo)
}

// Stop unregisters any active hotkey.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.mu.Unlock()
}

func parseCombination(input string) (combination, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(input)), "+")
	combo := combination{}
	for _, part := range parts {
		token := strings.TrimSpace(part)
		switch token {
		case "", "none":
			continue
		case "ctrl", "control":
			combo.Ctrl = true
		case "alt", "option", "opt":
			combo.Alt = true
		case "shift":
			combo.Shift = true
		case "cmd", "command", "meta", "super", "win", "windows":
			combo.Super = true
		default:
			if combo.Key != "" {
				return combination{}, fmt.Errorf("multiple keys specified: %s", token)
			}
			combo.Key = token
		}
	}
	if combo.Key == "" {
		return combination{}, errors.New("hotkey must include a key")
	}
	return combo, nil
}

func formatCombination(combo combination) string {
	parts := []string{}
	if combo.Ctrl {
		parts = append(parts, "ctrl")
	}
	if combo.Alt {
		parts = append(parts, "alt")
	}
	if combo.Shift {
		parts = append(parts, "shift")
	}
	if combo.Super {
		parts = append(parts, "super")
	}
	if combo.Key != "" {
		parts = append(parts, combo.Key)
	}
	return strings.Join(parts, "+")
}
