package main

import (
	"context"
	"database/sql"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"clipflow/internal/server/api"
	serverconfig "clipflow/internal/server/config"
	serverlog "clipflow/internal/server/log"
	"clipflow/internal/server/repo"
	"clipflow/internal/server/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := serverconfig.Load()
	logger := serverlog.Logger()

	// TODO: Register a SQLite driver (e.g., modernc.org/sqlite or mattn/go-sqlite3) before deploying.
	db, err := sql.Open("sqlite", cfg.DatabaseDSN)
	if err != nil {
		logger.Fatalf("failed to connect database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Printf("failed to close database: %v", err)
		}
	}()

	repository, err := repo.NewSQLClipboardRepository(db)
	if err != nil {
		logger.Fatalf("failed to initialize repository: %v", err)
	}

	syncService := service.NewSyncService(repository)
	handler := api.NewHandler(syncService)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler.Router(),
	}

	go func() {
		logger.Printf("server listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("failed to gracefully shutdown server: %v", err)
	}
}
