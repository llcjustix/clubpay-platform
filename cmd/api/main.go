package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	"clubpay/internal/db"
	"clubpay/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, "migrations/001_init.sql"); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	var coreAdapter core.Adapter = core.MockAdapter{}
	if strings.EqualFold(cfg.CoreMode, "http") {
		coreAdapter = core.NewHTTPAdapter(cfg.CoreBaseURL, cfg.CoreToken, time.Duration(cfg.CoreTimeoutMS)*time.Millisecond)
	}
	server := httpapi.NewServer(cfg, pool, coreAdapter)
	syncCtx, syncCancel := context.WithCancel(context.Background())
	defer syncCancel()
	go server.RunEdgeSync(syncCtx)
	go server.RunTelegramPolling(syncCtx)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("clubpay api listening on %s", cfg.HTTPAddr)
		errs <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Printf("received signal %s, shutting down", sig)
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
