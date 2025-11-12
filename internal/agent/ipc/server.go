package ipc

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clipflow/internal/agent/clipboard"
	agentconfig "clipflow/internal/agent/config"
	"clipflow/internal/agent/storage"
	agentsync "clipflow/internal/agent/sync"
)

// Server exposes local HTTP APIs for the desktop UI.
type Server struct {
	storage    storage.Storage
	adapter    clipboard.ClipboardAdapter
	config     *agentconfig.Manager
	httpSrv    *http.Server
	onConfigFn func(agentconfig.Config)
}

// NewServer constructs a new IPC server bound to the provided address.
func NewServer(addr string, store storage.Storage, adapter clipboard.ClipboardAdapter, cfg *agentconfig.Manager, onConfig func(agentconfig.Config)) *Server {
	s := &Server{storage: store, adapter: adapter, config: cfg, onConfigFn: onConfig}
	mux := http.NewServeMux()
	mux.HandleFunc("/items", s.routeItems)
	mux.HandleFunc("/items/", s.routeItemAction)
	mux.HandleFunc("/config", s.routeConfig)
	s.httpSrv = &http.Server{Addr: addr, Handler: mux}
	return s
}

// Start launches the HTTP server asynchronously.
func (s *Server) Start() error {
	go func() {
		_ = s.httpSrv.ListenAndServe()
	}()
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) routeItems(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListItems(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) routeItemAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/items/")
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	idSegment := segments[0]
	id, err := parseIDParam(idSegment)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if len(segments) == 1 && r.Method == http.MethodGet {
		s.handleGetItemByID(w, r, id)
		return
	}
	if len(segments) == 2 && r.Method == http.MethodPost {
		action := segments[1]
		s.handleAction(w, r, id, action)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) routeConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPost:
		s.handleUpdateConfig(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request, id int64, action string) {
	switch action {
	case "select":
		s.handleSelectItemByID(w, r, id)
	case "favorite":
		s.handleFavoriteItemByID(w, r, id)
	case "delete":
		s.handleDeleteItemByID(w, r, id)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	filter := storage.ListFilter{Query: r.URL.Query().Get("q"), Page: page, PageSize: pageSize}
	items, err := s.storage.ListItems(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responses := make([]clipboardItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, newClipboardItemResponse(item))
	}
	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) handleGetItemByID(w http.ResponseWriter, r *http.Request, id int64) {
	ctx := r.Context()
	item, err := s.storage.GetItem(ctx, id)
	if err != nil {
		if err == storage.ErrItemNotFound {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newClipboardItemResponse(*item))
}

func (s *Server) handleSelectItemByID(w http.ResponseWriter, r *http.Request, id int64) {
	ctx := r.Context()
	item, err := s.storage.GetItem(ctx, id)
	if err != nil {
		if err == storage.ErrItemNotFound {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	modelItem := agentsync.ConvertToModel(*item)
	if err := s.adapter.WriteContent(ctx, &modelItem); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFavoriteItemByID(w http.ResponseWriter, r *http.Request, id int64) {
	ctx := r.Context()
	var payload struct {
		Favorite bool `json:"favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.storage.SetFavorite(ctx, id, payload.Favorite); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteItemByID(w http.ResponseWriter, r *http.Request, id int64) {
	ctx := r.Context()
	if err := s.storage.SoftDelete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.config.Get()
	writeJSON(w, http.StatusOK, configResponseFromConfig(cfg))
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var payload configUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	cfg := s.config.Get()
	if payload.ServerURL != "" {
		cfg.ServerURL = payload.ServerURL
	}
	if payload.UserID != nil {
		cfg.UserID = *payload.UserID
	}
	if payload.DeviceID != nil {
		cfg.DeviceID = *payload.DeviceID
	}
	if payload.SyncIntervalSeconds != nil && *payload.SyncIntervalSeconds > 0 {
		cfg.SyncInterval = time.Duration(*payload.SyncIntervalSeconds) * time.Second
	}
	if err := s.config.Update(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.onConfigFn != nil {
		s.onConfigFn(cfg)
	}
	writeJSON(w, http.StatusOK, configResponseFromConfig(cfg))
}

func parseIDParam(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error":  message,
		"status": status,
		"time":   time.Now(),
	})
}

type clipboardItemResponse struct {
	LocalID     int64      `json:"localId"`
	RemoteID    string     `json:"remoteId,omitempty"`
	UserID      string     `json:"userId"`
	DeviceID    string     `json:"deviceId"`
	ContentType string     `json:"contentType"`
	Content     string     `json:"content"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	SyncedAt    *time.Time `json:"syncedAt,omitempty"`
	Favorite    bool       `json:"favorite"`
	Deleted     bool       `json:"deleted"`
}

type configResponse struct {
	ServerURL           string `json:"serverUrl"`
	UserID              string `json:"userId"`
	DeviceID            string `json:"deviceId"`
	SyncIntervalSeconds int    `json:"syncIntervalSeconds"`
}

type configUpdateRequest struct {
	ServerURL           string  `json:"serverUrl"`
	UserID              *string `json:"userId"`
	DeviceID            *string `json:"deviceId"`
	SyncIntervalSeconds *int    `json:"syncIntervalSeconds"`
}

func newClipboardItemResponse(item storage.LocalClipboardItem) clipboardItemResponse {
	var syncedAt *time.Time
	if item.SyncedAt.Valid {
		t := item.SyncedAt.Time
		syncedAt = &t
	}
	remoteID := ""
	if item.RemoteID.Valid {
		remoteID = item.RemoteID.String
	}
	return clipboardItemResponse{
		LocalID:     item.LocalID,
		RemoteID:    remoteID,
		UserID:      item.UserID,
		DeviceID:    item.DeviceID,
		ContentType: item.ContentType,
		Content:     item.Content,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		SyncedAt:    syncedAt,
		Favorite:    item.Favorite,
		Deleted:     item.IsDeleted,
	}
}

func configResponseFromConfig(cfg agentconfig.Config) configResponse {
	return configResponse{
		ServerURL:           cfg.ServerURL,
		UserID:              cfg.UserID,
		DeviceID:            cfg.DeviceID,
		SyncIntervalSeconds: int(cfg.SyncInterval / time.Second),
	}
}
