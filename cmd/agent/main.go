package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"

	agentclipboard "clipflow/internal/agent/clipboard"
	agentconfig "clipflow/internal/agent/config"
	agentipc "clipflow/internal/agent/ipc"
	agentlog "clipflow/internal/agent/log"
	agentstorage "clipflow/internal/agent/storage"
	agentsync "clipflow/internal/agent/sync"
	"clipflow/internal/common/model"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := agentlog.Logger()

	cfgPath := agentconfig.DefaultPath()
	cfg, err := agentconfig.LoadOrCreate(cfgPath)
	if err != nil {
		logger.Printf("failed to load agent config from disk, falling back to environment: %v", err)
		cfg = agentconfig.Load()
	}
	cfgManager := agentconfig.NewManager(cfgPath, cfg)

	store, err := agentstorage.NewSQLiteStorage(cfg.DatabasePath)
	if err != nil {
		logger.Fatalf("failed to open storage: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Printf("failed to close storage: %v", err)
		}
	}()

	adapter := agentclipboard.NewMacOSAdapter()
	syncManager := agentsync.NewManager(cfg, store)

	var (
		cfgMu        sync.RWMutex
		currentCfg   = cfg
		configUpdate = make(chan agentconfig.Config, 1)
	)

	setCurrentCfg := func(newCfg agentconfig.Config) {
		cfgMu.Lock()
		currentCfg = newCfg
		cfgMu.Unlock()
	}
	getCurrentCfg := func() agentconfig.Config {
		cfgMu.RLock()
		defer cfgMu.RUnlock()
		return currentCfg
	}

	ipcServer := agentipc.NewServer(cfg.IPCAddress, store, adapter, cfgManager, func(newCfg agentconfig.Config) {
		select {
		case configUpdate <- newCfg:
		default:
		}
	})
	if err := ipcServer.Start(); err != nil {
		logger.Fatalf("failed to start IPC server: %v", err)
	}
	logger.Printf("IPC server listening on %s", cfg.IPCAddress)

	go func() {
		err := adapter.Watch(ctx, func(ctx context.Context, item *model.ClipboardItem) error {
			if item == nil {
				return nil
			}
			now := time.Now()
			cfgSnapshot := getCurrentCfg()
			local := agentstorage.LocalClipboardItem{
				UserID:      cfgSnapshot.UserID,
				DeviceID:    cfgSnapshot.DeviceID,
				ContentType: item.ContentType,
				Content:     item.Content,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			return store.SaveNewItem(ctx, &local)
		})
		if err != nil {
			logger.Printf("clipboard watch terminated: %v", err)
		}
	}()

	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()

	runSync := func() {
		if err := syncManager.PushLocalChanges(ctx); err != nil {
			logger.Printf("push sync error: %v", err)
		}
		if err := syncManager.PullRemoteChanges(ctx); err != nil {
			logger.Printf("pull sync error: %v", err)
		}
	}

	runSync()

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := ipcServer.Shutdown(shutdownCtx); err != nil {
				logger.Printf("failed to shutdown IPC server: %v", err)
			}
			return
		case <-ticker.C:
			runSync()
		case newCfg := <-configUpdate:
			syncManager.UpdateConfig(newCfg)
			setCurrentCfg(newCfg)
			ticker.Reset(newCfg.SyncInterval)
			logger.Printf("agent configuration updated: server=%s interval=%s", newCfg.ServerURL, newCfg.SyncInterval)
		}
	}
}
