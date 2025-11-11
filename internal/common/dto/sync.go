package dto

import "time"

// UploadChangesRequest represents the payload sent by the agent when pushing local changes.
type UploadChangesRequest struct {
	Items            []ClipboardSyncUploadItem `json:"items"`
	LastKnownVersion int64                     `json:"lastKnownVersion"`
}

// ClipboardSyncUploadItem conveys a single clipboard item mutation from the agent to the server.
type ClipboardSyncUploadItem struct {
	LocalID     string    `json:"localId"`
	RemoteID    string    `json:"remoteId,omitempty"`
	UserID      string    `json:"userId"`
	DeviceID    string    `json:"deviceId"`
	ContentType string    `json:"contentType"`
	Content     string    `json:"content"`
	Favorite    bool      `json:"favorite"`
	Deleted     bool      `json:"deleted"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// UploadChangesResponse represents the mapping from local identifiers to remote identifiers.
type UploadChangesResponse struct {
	Results        []ClipboardSyncUploadResult `json:"results"`
	CurrentVersion int64                       `json:"currentVersion"`
}

// ClipboardSyncUploadResult contains the synchronization result for a single item.
type ClipboardSyncUploadResult struct {
	LocalID  string `json:"localId"`
	RemoteID string `json:"remoteId"`
	Version  int64  `json:"version"`
}

// GetChangesResponse encapsulates the incremental changes returned by the server.
type GetChangesResponse struct {
	Items          []ClipboardItemChange `json:"items"`
	CurrentVersion int64                 `json:"currentVersion"`
}

// ClipboardItemChange describes a remote clipboard item and its state.
type ClipboardItemChange struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	DeviceID    string    `json:"deviceId,omitempty"`
	ContentType string    `json:"contentType"`
	Content     string    `json:"content"`
	Favorite    bool      `json:"favorite"`
	Deleted     bool      `json:"deleted"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Version     int64     `json:"version"`
}
