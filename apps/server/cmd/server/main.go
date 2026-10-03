package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"graduation/apps/server/internal/api"
	"graduation/apps/server/internal/auth"
	"graduation/apps/server/internal/config"
	"graduation/apps/server/internal/database"
	"graduation/apps/server/internal/room"
	ws "graduation/apps/server/internal/websocket"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	store, err := database.Open(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	err = store.Migrate(ctx)
	cancel()
	if err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	if err = bootstrapAdmin(store, cfg.BootstrapEmail, cfg.BootstrapPassword); err != nil {
		slog.Error("admin bootstrap failed", "error", err)
		os.Exit(1)
	}
	records, err := store.ListRooms(context.Background())
	if err != nil {
		slog.Error("load rooms failed", "error", err)
		os.Exit(1)
	}
	authService := auth.New(store, cfg.SessionTTL, cfg.CookieSecure)
	manager := room.NewManager(records, store, cfg.BatchInterval, cfg.PresenceInterval, cfg.StatsFlushInterval)
	for _, record := range records {
		metrics, metricsErr := store.RoomMetrics(context.Background(), record.ID)
		if metricsErr != nil {
			slog.Error("load room metrics failed", "room", record.Code, "error", metricsErr)
			os.Exit(1)
		}
		points, statsErr := store.Stats(context.Background(), record.ID)
		if statsErr != nil {
			slog.Error("load reaction statistics failed", "room", record.Code, "error", statsErr)
			os.Exit(1)
		}
		manager.SeedStatistics(record.Code, metrics, points)
	}
	wsHandler := ws.New(manager, store, authService, cfg.FrontendURL)
	apiServer := api.New(store, manager, authService, wsHandler, cfg.FrontendURL)
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	go manager.Start(runtimeCtx)
	server := &http.Server{Addr: cfg.Address, Handler: apiServer.Router(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, WriteTimeout: 15 * time.Second}
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("server listening", "address", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signals:
		slog.Info("shutdown signal received")
	case listenErr := <-serverErrors:
		if !errors.Is(listenErr, http.ErrServerClosed) {
			slog.Error("server stopped unexpectedly", "error", listenErr)
		}
	}
	slog.Info("graceful shutdown started")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	manager.Shutdown(shutdownCtx)
	runtimeCancel()
	slog.Info("shutdown complete")
}
func bootstrapAdmin(store *database.Store, email, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	count, err := store.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if email == "" || password == "" {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD are required when no admin exists")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err = store.CreateAdmin(ctx, email, hash); err == nil {
		slog.Info("bootstrap admin created", "email", email)
	}
	return err
}
