package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	"clubpay/internal/db"
	"clubpay/internal/envfile"
	"clubpay/internal/httpapi"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	configPath := flag.String("config", defaultControllerConfigPath(), "path to controller.env")
	install := flag.Bool("install", false, "install Controller Node as a Windows startup task")
	uninstall := flag.Bool("uninstall", false, "remove the Controller Node Windows startup task")
	flag.Parse()
	if *install && *uninstall {
		log.Fatal("use either --install or --uninstall")
	}
	if *uninstall {
		if err := uninstallWindowsControllerTask(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := envfile.LoadIntoEnvironment(*configPath); err != nil {
		log.Fatalf("load controller config: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.WebDir != "" {
		cfg.WebDir = resolvePathFromConfig(cfg.WebDir, *configPath)
	}
	if *install {
		if err := validateControllerNodeConfig(cfg); err != nil {
			log.Fatal(err)
		}
		if err := installWindowsControllerTask(*configPath); err != nil {
			log.Fatal(err)
		}
		log.Printf("ClubPay Controller Node will start automatically with Windows")
		return
	}

	ctx := context.Background()
	pool, err := connectDatabaseWithRetry(ctx, cfg.DatabaseURL, 90*time.Second)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, resolveMigrationsDir()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	var coreAdapter core.Adapter = core.NewMockAdapter()
	var wsController *core.WSController
	var edgeWOLRelay *core.EdgeWOLRelay
	var wakeHandler core.WakeHandler
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
			wakeHandler = func(ctx context.Context, externalPCID string) error {
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
			}
		} else if edgeWOLRelay != nil {
			wakeHandler = func(ctx context.Context, externalPCID string) error {
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
			}
		} else {
			log.Printf("Wake-on-LAN is enabled but EDGE_WOL_TOKEN is not configured; sleeping PCs cannot be awakened")
		}
	}
	if wsController != nil && wakeHandler != nil {
		wsController.SetWakeHandler(wakeHandler, time.Duration(cfg.WOLWaitSeconds)*time.Second)
	}
	server := httpapi.NewServer(cfg, pool, coreAdapter)
	if wakeHandler != nil {
		server.SetWakePCHandler(wakeHandler)
	}
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

func defaultControllerConfigPath() string {
	executable, err := os.Executable()
	if err != nil {
		return "controller.env"
	}
	return filepath.Join(filepath.Dir(executable), "controller.env")
}

func resolveMigrationsDir() string {
	if configured := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); configured != "" {
		return configured
	}
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "migrations")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
	}
	return "migrations"
}

func resolvePathFromConfig(value, configPath string) string {
	if filepath.IsAbs(value) {
		return value
	}
	if absoluteConfigPath, err := filepath.Abs(configPath); err == nil {
		return filepath.Join(filepath.Dir(absoluteConfigPath), value)
	}
	return value
}

func validateControllerNodeConfig(cfg config.Config) error {
	if !strings.EqualFold(cfg.NodeMode, "edge") && !strings.EqualFold(cfg.NodeMode, "manager") {
		return errors.New("NODE_MODE must be edge or manager for a local Controller Node")
	}
	if strings.TrimSpace(cfg.EdgeClubID) == "" {
		return errors.New("EDGE_CLUB_ID (or MANAGER_CLUB_ID) is required")
	}
	if strings.TrimSpace(cfg.CloudBaseURL) == "" {
		return errors.New("CLOUD_BASE_URL is required for initial synchronization")
	}
	if strings.TrimSpace(cfg.EdgeSyncToken) == "" {
		return errors.New("EDGE_SYNC_TOKEN is required for initial synchronization")
	}
	if strings.TrimSpace(cfg.WebDir) == "" {
		return errors.New("WEB_DIR is required; it must point to the packaged PWA folder")
	}
	return nil
}

func installWindowsControllerTask(configPath string) error {
	if runtime.GOOS != "windows" {
		return errors.New("--install is available only on Windows; use systemd on Linux")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	command := fmt.Sprintf("\"%s\" --config \"%s\"", executable, configPath)
	result, err := exec.Command("schtasks.exe", "/Create", "/TN", "ClubPay Controller Node", "/SC", "ONSTART", "/RU", "SYSTEM", "/RL", "HIGHEST", "/TR", command, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("create Windows startup task: %w: %s", err, strings.TrimSpace(string(result)))
	}
	return nil
}

func uninstallWindowsControllerTask() error {
	if runtime.GOOS != "windows" {
		return errors.New("--uninstall is available only on Windows")
	}
	result, err := exec.Command("schtasks.exe", "/Delete", "/TN", "ClubPay Controller Node", "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove Windows startup task: %w: %s", err, strings.TrimSpace(string(result)))
	}
	return nil
}

func connectDatabaseWithRetry(ctx context.Context, databaseURL string, timeout time.Duration) (*pgxpool.Pool, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		pool, err := db.Connect(ctx, databaseURL)
		if err == nil {
			return pool, nil
		}
		lastErr = err
		log.Printf("waiting for local PostgreSQL: %v", err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("connect database after %s: %w", timeout, lastErr)
}
