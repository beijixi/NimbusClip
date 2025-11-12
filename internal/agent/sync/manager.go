package sync

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	agentconfig "clipflow/internal/agent/config"
	"clipflow/internal/agent/storage"
	"clipflow/internal/common/dto"
	"clipflow/internal/common/model"
)

// Manager coordinates synchronization between the local agent and the remote server.
type Manager struct {
	client *http.Client

	mu     sync.RWMutex
	config agentconfig.Config

	storage storage.Storage
	state   *State
}

// State keeps track of synchronization cursors.
type State struct {
	LastKnownVersion int64
}

// NewManager constructs a new synchronization manager.
func NewManager(cfg agentconfig.Config, storage storage.Storage) *Manager {
	return &Manager{
		client:  &http.Client{Timeout: 15 * time.Second},
		config:  cfg,
		storage: storage,
		state:   &State{},
	}
}

// UpdateConfig swaps the runtime configuration and resets version tracking.
func (m *Manager) UpdateConfig(cfg agentconfig.Config) {
	m.mu.Lock()
	m.config = cfg
	m.state.LastKnownVersion = 0
	m.mu.Unlock()
}

// PushLocalChanges uploads unsynchronized local items to the server.
func (m *Manager) PushLocalChanges(ctx context.Context) error {
	items, err := m.storage.ListUnsyncedItems(ctx, 200)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	cfg := m.currentConfig()
	uploadItems := make([]dto.ClipboardSyncUploadItem, 0, len(items))
	for _, item := range items {
		uploadItems = append(uploadItems, dto.ClipboardSyncUploadItem{
			LocalID:     fmt.Sprintf("%d", item.LocalID),
			RemoteID:    nullString(item.RemoteID),
			UserID:      item.UserID,
			DeviceID:    item.DeviceID,
			ContentType: item.ContentType,
			Content:     item.Content,
			Favorite:    item.Favorite,
			Deleted:     item.IsDeleted,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	req := dto.UploadChangesRequest{
		Items:            uploadItems,
		LastKnownVersion: m.state.LastKnownVersion,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/clipboard/sync/upload", cfg.ServerURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	m.applyHeaders(httpReq, cfg)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("push changes failed: %s", resp.Status)
	}
	var uploadResp dto.UploadChangesResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return err
	}
	for _, result := range uploadResp.Results {
		localID, err := parseLocalID(result.LocalID)
		if err != nil {
			continue
		}
		if err := m.storage.MarkAsSynced(ctx, localID, result.RemoteID, time.Now()); err != nil {
			return err
		}
	}
	m.state.LastKnownVersion = uploadResp.CurrentVersion
	return nil
}

// PullRemoteChanges fetches remote changes since the last known version.
func (m *Manager) PullRemoteChanges(ctx context.Context) error {
	cfg := m.currentConfig()
	url := fmt.Sprintf("%s/api/v1/clipboard/sync/changes?lastVersion=%d", cfg.ServerURL, m.state.LastKnownVersion)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	m.applyHeaders(httpReq, cfg)
	resp, err := m.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("pull changes failed: %s", resp.Status)
	}
	var changes dto.GetChangesResponse
	if err := json.NewDecoder(resp.Body).Decode(&changes); err != nil {
		return err
	}
	for _, item := range changes.Items {
		remote := storage.RemoteClipboardItem{
			RemoteID:    item.ID,
			UserID:      item.UserID,
			DeviceID:    item.DeviceID,
			ContentType: item.ContentType,
			Content:     item.Content,
			Favorite:    item.Favorite,
			Deleted:     item.Deleted,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
		if err := m.storage.UpsertFromRemote(ctx, remote); err != nil {
			return err
		}
	}
	m.state.LastKnownVersion = changes.CurrentVersion
	return nil
}

func (m *Manager) applyHeaders(req *http.Request, cfg agentconfig.Config) {
	req.Header.Set("X-User-Id", cfg.UserID)
	req.Header.Set("X-Device-Id", cfg.DeviceID)
}

func (m *Manager) currentConfig() agentconfig.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func nullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func parseLocalID(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}

// ConvertToModel transforms a local clipboard item into a shared model representation.
func ConvertToModel(item storage.LocalClipboardItem) model.ClipboardItem {
	return model.ClipboardItem{
		ID:          nullString(item.RemoteID),
		UserID:      item.UserID,
		DeviceID:    item.DeviceID,
		ContentType: item.ContentType,
		Content:     item.Content,
		Favorite:    item.Favorite,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}
