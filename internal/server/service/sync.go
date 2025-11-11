package service

import (
	"context"

	"clipflow/internal/common/dto"
	"clipflow/internal/common/model"
	"clipflow/internal/server/auth"
	"clipflow/internal/server/repo"
	serversync "clipflow/internal/server/sync"
)

// SyncService coordinates clipboard synchronization requests.
type SyncService struct {
	repo  repo.ClipboardRepository
	clock serversync.Clock
}

// NewSyncService constructs a new SyncService instance.
func NewSyncService(repository repo.ClipboardRepository) *SyncService {
	return &SyncService{repo: repository, clock: serversync.SystemClock{}}
}

// WithClock overrides the clock implementation used by the service.
func (s *SyncService) WithClock(clock serversync.Clock) *SyncService {
	s.clock = clock
	return s
}

// UploadChanges merges client-side changes into the server and returns mapping details.
func (s *SyncService) UploadChanges(ctx context.Context, principal auth.Principal, req dto.UploadChangesRequest) (dto.UploadChangesResponse, error) {
	results := make([]dto.ClipboardSyncUploadResult, 0, len(req.Items))
	currentVersion := req.LastKnownVersion
	for _, item := range req.Items {
		modelItem := model.ClipboardItem{
			ID:          item.RemoteID,
			UserID:      principal.UserID,
			DeviceID:    principal.DeviceID,
			ContentType: item.ContentType,
			Content:     item.Content,
			Favorite:    item.Favorite,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
		if item.Deleted {
			now := item.UpdatedAt
			if now.IsZero() {
				now = s.clock.Now()
			}
			modelItem.DeletedAt = &now
		}
		persisted, err := s.repo.CreateOrUpdateClipboardItem(ctx, modelItem)
		if err != nil {
			return dto.UploadChangesResponse{}, err
		}
		if persisted.Version > currentVersion {
			currentVersion = persisted.Version
		}
		results = append(results, dto.ClipboardSyncUploadResult{
			LocalID:  item.LocalID,
			RemoteID: persisted.ID,
			Version:  persisted.Version,
		})
	}
	return dto.UploadChangesResponse{Results: results, CurrentVersion: currentVersion}, nil
}

// GetChangesSince retrieves incremental clipboard changes for the client.
func (s *SyncService) GetChangesSince(ctx context.Context, principal auth.Principal, lastVersion int64) (dto.GetChangesResponse, error) {
	items, err := s.repo.ListChangesSinceVersion(ctx, principal.UserID, lastVersion)
	if err != nil {
		return dto.GetChangesResponse{}, err
	}
	changes := make([]dto.ClipboardItemChange, 0, len(items))
	currentVersion := lastVersion
	for _, item := range items {
		changes = append(changes, dto.ClipboardItemChange{
			ID:          item.ID,
			UserID:      item.UserID,
			DeviceID:    item.DeviceID,
			ContentType: item.ContentType,
			Content:     item.Content,
			Favorite:    item.Favorite,
			Deleted:     item.IsDeleted(),
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
			Version:     item.Version,
		})
		if item.Version > currentVersion {
			currentVersion = item.Version
		}
	}
	return dto.GetChangesResponse{Items: changes, CurrentVersion: currentVersion}, nil
}
