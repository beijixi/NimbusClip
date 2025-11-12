package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client wraps HTTP access to the local agent IPC server.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// NewClient constructs a new agent IPC client.
func NewClient(base string) (*Client, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	return &Client{
		baseURL:    parsed,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// SetBase updates the client endpoint.
func (c *Client) SetBase(base string) error {
	parsed, err := url.Parse(base)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	c.baseURL = parsed
	return nil
}

// ClipboardItem represents a clipboard record returned by the agent.
type ClipboardItem struct {
	LocalID     int64      `json:"localId"`
	RemoteID    string     `json:"remoteId"`
	UserID      string     `json:"userId"`
	DeviceID    string     `json:"deviceId"`
	ContentType string     `json:"contentType"`
	Content     string     `json:"content"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	SyncedAt    *time.Time `json:"syncedAt"`
	Favorite    bool       `json:"favorite"`
	Deleted     bool       `json:"deleted"`
}

// ConfigSnapshot describes the agent runtime configuration returned via IPC.
type ConfigSnapshot struct {
	ServerURL           string `json:"serverUrl"`
	UserID              string `json:"userId"`
	DeviceID            string `json:"deviceId"`
	SyncIntervalSeconds int    `json:"syncIntervalSeconds"`
}

// UpdateConfigRequest carries configuration mutations to the agent.
type UpdateConfigRequest struct {
	ServerURL           string  `json:"serverUrl"`
	UserID              *string `json:"userId"`
	DeviceID            *string `json:"deviceId"`
	SyncIntervalSeconds *int    `json:"syncIntervalSeconds"`
}

// ListItems retrieves clipboard entries.
func (c *Client) ListItems(ctx context.Context, query string, page, pageSize int) ([]ClipboardItem, error) {
	endpoint := c.resolve("items")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	if query != "" {
		q.Set("q", query)
	}
	if page > 0 {
		q.Set("page", fmt.Sprintf("%d", page))
	}
	if pageSize > 0 {
		q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list items failed: %s", resp.Status)
	}
	var items []ClipboardItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// GetItem retrieves a single clipboard item by id.
func (c *Client) GetItem(ctx context.Context, id int64) (*ClipboardItem, error) {
	endpoint := c.resolve(fmt.Sprintf("items/%d", id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get item failed: %s", resp.Status)
	}
	var item ClipboardItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

// SelectItem requests the agent to copy the item into the active clipboard.
func (c *Client) SelectItem(ctx context.Context, id int64) error {
	return c.postAction(ctx, id, "select", nil)
}

// Favorite toggles the favorite state for an item.
func (c *Client) Favorite(ctx context.Context, id int64, favorite bool) error {
	payload := map[string]any{"favorite": favorite}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.postAction(ctx, id, "favorite", body)
}

// Delete marks an item as deleted.
func (c *Client) Delete(ctx context.Context, id int64) error {
	return c.postAction(ctx, id, "delete", nil)
}

// GetConfig fetches the current agent configuration snapshot.
func (c *Client) GetConfig(ctx context.Context) (ConfigSnapshot, error) {
	endpoint := c.resolve("config")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return ConfigSnapshot{}, fmt.Errorf("get config failed: %s", resp.Status)
	}
	var cfg ConfigSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return ConfigSnapshot{}, err
	}
	return cfg, nil
}

// UpdateConfig sends configuration changes to the agent.
func (c *Client) UpdateConfig(ctx context.Context, payload UpdateConfigRequest) (ConfigSnapshot, error) {
	endpoint := c.resolve("config")
	body, err := json.Marshal(payload)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ConfigSnapshot{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return ConfigSnapshot{}, fmt.Errorf("update config failed: %s", resp.Status)
	}
	var cfg ConfigSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return ConfigSnapshot{}, err
	}
	return cfg, nil
}

func (c *Client) postAction(ctx context.Context, id int64, action string, body []byte) error {
	endpoint := c.resolve(fmt.Sprintf("items/%d/%s", id, action))
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("action %s failed: %s", action, resp.Status)
	}
	return nil
}

func (c *Client) resolve(p string) string {
	relPath := strings.TrimPrefix(p, "/")
	ref := &url.URL{Path: "/" + relPath}
	return c.baseURL.ResolveReference(ref).String()
}
