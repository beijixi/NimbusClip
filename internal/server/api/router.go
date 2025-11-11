package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"clipflow/internal/common/dto"
	"clipflow/internal/server/auth"
	"clipflow/internal/server/service"
)

// Handler exposes HTTP handlers for clipboard synchronization.
type Handler struct {
	syncService *service.SyncService
}

// NewHandler creates a new Handler instance.
func NewHandler(syncService *service.SyncService) *Handler {
	return &Handler{syncService: syncService}
}

// Router builds the HTTP router for the clipboard API.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	upload := h.wrapAuth(http.HandlerFunc(h.handleUpload))
	changes := h.wrapAuth(http.HandlerFunc(h.handleChanges))
	mux.Handle("/api/v1/clipboard/sync/upload", upload)
	mux.Handle("/api/v1/clipboard/sync/changes", changes)
	return mux
}

func (h *Handler) wrapAuth(next http.Handler) http.Handler {
	return auth.Middleware(next)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req dto.UploadChangesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	resp, err := h.syncService.UploadChanges(r.Context(), principal, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lastVersion := int64(0)
	if v := r.URL.Query().Get("lastVersion"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lastVersion")
			return
		}
		lastVersion = parsed
	}
	resp, err := h.syncService.GetChangesSince(r.Context(), principal, lastVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
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
	})
}
