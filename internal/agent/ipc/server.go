package ipc

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clipflow/internal/agent/clipboard"
	"clipflow/internal/agent/storage"
	"clipflow/internal/agent/sync"
)

// Server exposes local HTTP APIs for the desktop UI.
type Server struct {
	storage storage.Storage
	adapter clipboard.ClipboardAdapter
	httpSrv *http.Server
}

// NewServer constructs a new IPC server bound to the provided address.
func NewServer(addr string, store storage.Storage, adapter clipboard.ClipboardAdapter) *Server {
	s := &Server{storage: store, adapter: adapter}
	mux := http.NewServeMux()
	mux.HandleFunc("/items", s.routeItems)
	mux.HandleFunc("/items/", s.routeItemAction)
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
	writeJSON(w, http.StatusOK, items)
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
	writeJSON(w, http.StatusOK, item)
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
	modelItem := sync.ConvertToModel(*item)
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
