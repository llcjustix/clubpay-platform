package main

import (
	"context"
	"fmt"
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

	if err := db.RunMigrations(ctx, pool, "migrations"); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	var coreAdapter core.Adapter = core.NewMockAdapter()
	var wsController *core.WSController
	var edgeWOLRelay *core.EdgeWOLRelay
	if strings.EqualFold(cfg.CoreMode, "http") {
		coreAdapter = core.NewHTTPAdapter(cfg.CoreBaseURL, cfg.CoreToken, time.Duration(cfg.CoreTimeoutMS)*time.Millisecond)
	} else if strings.EqualFold(cfg.CoreMode, "ws") || strings.EqualFold(cfg.CoreMode, "websocket") {
		wsController = core.NewWSController(cfg.CoreToken, time.Duration(cfg.CoreTimeoutMS)*time.Millisecond)
		coreAdapter = wsController
	}
	if strings.EqualFold(cfg.NodeMode, "cloud") && strings.TrimSpace(cfg.EdgeWOLToken) != "" {
		edgeWOLRelay = core.NewEdgeWOLRelay(cfg.EdgeWOLToken, time.Duration(cfg.CoreTimeoutMS)*time.Millisecond)
	}
	if wsController != nil && cfg.WOLEnabled {
		if cfg.NodeMode == "edge" || cfg.NodeMode == "manager" {
			wsController.SetWakeHandler(func(ctx context.Context, externalPCID string) error {
				var macAddress string
				err := pool.QueryRow(ctx, `
					SELECT COALESCE(mac_address, '')
					FROM pc_refs
					WHERE external_pc_id = $1
					  AND status_cache <> 'deleted'
					  AND (NULLIF($2, '')::uuid IS NULL OR club_id = NULLIF($2, '')::uuid)
					ORDER BY created_at DESC
					LIMIT 1
				`, externalPCID, cfg.EdgeClubID).Scan(&macAddress)
				if err != nil {
					return fmt.Errorf("lookup PC MAC: %w", err)
				}
				if strings.TrimSpace(macAddress) == "" {
					return fmt.Errorf("PC %s has no MAC address configured", externalPCID)
				}
				return core.SendWakeOnLAN(ctx, macAddress, cfg.WOLBroadcastAddr)
			}, time.Duration(cfg.WOLWaitSeconds)*time.Second)
		} else if edgeWOLRelay != nil {
			wsController.SetWakeHandler(func(ctx context.Context, externalPCID string) error {
				var clubID, macAddress string
				err := pool.QueryRow(ctx, `
					SELECT club_id::text, COALESCE(mac_address, '')
					FROM pc_refs
					WHERE external_pc_id = $1 AND status_cache <> 'deleted'
					ORDER BY created_at DESC
					LIMIT 1
				`, externalPCID).Scan(&clubID, &macAddress)
				if err != nil {
					return fmt.Errorf("lookup PC MAC: %w", err)
				}
				if strings.TrimSpace(macAddress) == "" {
					return fmt.Errorf("PC %s has no MAC address configured", externalPCID)
				}
				return edgeWOLRelay.Wake(ctx, clubID, externalPCID, macAddress)
			}, time.Duration(cfg.WOLWaitSeconds)*time.Second)
		} else {
			log.Printf("Wake-on-LAN is enabled but EDGE_WOL_TOKEN is not configured; sleeping PCs cannot be awakened")
		}
	}
	server := httpapi.NewServer(cfg, pool, coreAdapter)
	if edgeWOLRelay != nil {
		server.SetEdgeWOLRelay(edgeWOLRelay)
	}
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
