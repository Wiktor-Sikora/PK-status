package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Wiktor-Sikora/pk-status/backend/internal/api"
	"github.com/Wiktor-Sikora/pk-status/backend/internal/checker"
	"github.com/Wiktor-Sikora/pk-status/backend/internal/config"
	"github.com/Wiktor-Sikora/pk-status/backend/internal/storage"
)

func main() {
	cfg := config.Load()
	log.Printf("pk-status: targets=%v interval=%s db=%s port=%s retention raw=%s hourly=%s daily=%s",
		cfg.Targets, cfg.PollInterval, cfg.DBPath, cfg.Port, cfg.RawRetention, cfg.HourlyRetention, cfg.DailyRetention)

	store, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	scheduler := checker.NewScheduler(cfg.URLs(), cfg.PollInterval, store)
	go scheduler.Run(ctx)
	go store.RunRetention(ctx, time.Hour, storage.Retention{
		Raw:    cfg.RawRetention,
		Hourly: cfg.HourlyRetention,
		Daily:  cfg.DailyRetention,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: api.Handler(store, cfg.Names()),
	}

	go func() {
		log.Printf("pk-status: listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("pk-status: shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
}
