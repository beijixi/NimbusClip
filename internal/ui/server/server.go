package server

import (
	"context"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"

	agentclient "clipflow/internal/ui/client/agent"
	uiconfig "clipflow/internal/ui/config"
	"clipflow/internal/ui/hotkey"
)

// Server hosts the local browser-based desktop interface.
type Server struct {
	addr       string
	httpServer *http.Server

	agent      *agentclient.Client
	configPath string

	mu       sync.RWMutex
	uiConfig uiconfig.Config

	hotkeys         *hotkey.Manager
	hotkeySupported bool
	hotkeyError     string

	onHotkey func()
	tmpl     *template.Template
}

// New constructs a UI server bound to the provided address.
func New(addr string, cfg uiconfig.Config, cfgPath string, client *agentclient.Client, hk *hotkey.Manager, onHotkey func()) (*Server, error) {
	tpl, err := template.ParseFS(templateFiles, "templates/index.html")
	if err != nil {
		return nil, err
	}
	srv := &Server{
		addr:       addr,
		agent:      client,
		configPath: cfgPath,
		uiConfig:   cfg,
		hotkeys:    hk,
		onHotkey:   onHotkey,
		tmpl:       tpl,
	}
	if hk != nil {
		if err := hk.Set(cfg.WindowHotkey, srv.triggerHotkey); err != nil {
			srv.hotkeySupported = false
			srv.hotkeyError = err.Error()
		} else {
			srv.hotkeySupported = true
			srv.hotkeyError = ""
		}
	}
	return srv, nil
}

// Start launches the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/history/", s.handleHistoryAction)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/hotkey", s.handleHotkey)

	assets, err := fs.Sub(assetFiles, "assets")
	if err != nil {
		return err
	}
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))

	s.httpServer = &http.Server{Addr: s.addr, Handler: mux}
	go func() {
		_ = s.httpServer.ListenAndServe()
	}()
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.hotkeys != nil {
		s.hotkeys.Stop()
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	data := struct {
		Hotkey          string
		HotkeySupported bool
		HotkeyError     string
	}{
		Hotkey:          s.uiConfig.WindowHotkey,
		HotkeySupported: s.hotkeySupported,
		HotkeyError:     s.hotkeyError,
	}
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	favoritesOnly := r.URL.Query().Get("favorites") == "1"
	s.mu.RLock()
	pageSize := s.uiConfig.PageSize
	s.mu.RUnlock()
	items, err := s.agent.ListItems(ctx, query, 0, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if favoritesOnly {
		filtered := make([]agentclient.ClipboardItem, 0, len(items))
		for _, item := range items {
			if item.Favorite {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleHistoryAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/history/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	action := parts[1]
	ctx := r.Context()
	switch action {
	case "select":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.agent.SelectItem(ctx, id); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "favorite":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Favorite bool `json:"favorite"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if err := s.agent.Favorite(ctx, id, payload.Favorite); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "delete":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.agent.Delete(ctx, id); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleConfigGet(w, r)
	case http.MethodPost:
		s.handleConfigPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.agent.GetConfig(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.mu.RLock()
	uiCfg := s.uiConfig
	s.mu.RUnlock()
	resp := map[string]any{
		"agentEndpoint":       uiCfg.AgentEndpoint,
		"serverUrl":           cfg.ServerURL,
		"userId":              cfg.UserID,
		"deviceId":            cfg.DeviceID,
		"syncIntervalSeconds": cfg.SyncIntervalSeconds,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleConfigPost(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		AgentEndpoint      string `json:"agentEndpoint"`
		ServerURL          string `json:"serverUrl"`
		UserID             string `json:"userId"`
		DeviceID           string `json:"deviceId"`
		SyncIntervalSecond int    `json:"syncIntervalSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	req := agentclient.UpdateConfigRequest{ServerURL: payload.ServerURL}
	if payload.UserID != "" {
		req.UserID = &payload.UserID
	}
	if payload.DeviceID != "" {
		req.DeviceID = &payload.DeviceID
	}
	if payload.SyncIntervalSecond > 0 {
		req.SyncIntervalSeconds = &payload.SyncIntervalSecond
	}
	cfg, err := s.agent.UpdateConfig(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.mu.Lock()
	if payload.AgentEndpoint != "" {
		if err := s.agent.SetBase(payload.AgentEndpoint); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.uiConfig.AgentEndpoint = payload.AgentEndpoint
	}
	if payload.SyncIntervalSecond <= 0 {
		payload.SyncIntervalSecond = cfg.SyncIntervalSeconds
	}
	if err := uiconfig.SaveToFile(s.configPath, s.uiConfig); err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"agentEndpoint":       payload.AgentEndpoint,
		"serverUrl":           cfg.ServerURL,
		"userId":              cfg.UserID,
		"deviceId":            cfg.DeviceID,
		"syncIntervalSeconds": cfg.SyncIntervalSeconds,
	})
}

func (s *Server) handleHotkey(w http.ResponseWriter, r *http.Request) {
	if s.hotkeys == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"combo":     "",
			"supported": false,
			"error":     "hotkeys unavailable",
		})
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		combo := s.uiConfig.WindowHotkey
		supported := s.hotkeySupported
		errMsg := s.hotkeyError
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"combo":     combo,
			"supported": supported,
			"error":     errMsg,
		})
	case http.MethodPost:
		var payload struct {
			Combo string `json:"combo"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		combo := strings.TrimSpace(payload.Combo)
		if combo == "" {
			http.Error(w, "hotkey cannot be empty", http.StatusBadRequest)
			return
		}
		if err := s.hotkeys.Set(combo, s.triggerHotkey); err != nil {
			s.mu.Lock()
			s.hotkeySupported = false
			s.hotkeyError = err.Error()
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.hotkeySupported = true
		s.hotkeyError = ""
		s.uiConfig.WindowHotkey = combo
		_ = uiconfig.SaveToFile(s.configPath, s.uiConfig)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"combo":     combo,
			"supported": true,
			"error":     "",
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) triggerHotkey() {
	if s.onHotkey != nil {
		s.onHotkey()
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
