package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"clubpay/internal/config"
	"clubpay/internal/core"
	"clubpay/internal/db"
	"clubpay/internal/envfile"
	"clubpay/internal/httpapi"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	configPath := flag.String("config", defaultControllerConfigPath(), "path to controller.env")
	install := flag.Bool("install", false, "install Controller Node as a Windows startup task")
	uninstall := flag.Bool("uninstall", false, "remove the Controller Node Windows startup task")
	setup := flag.Bool("setup", false, "enroll this Controller Node with a one-time activation code")
	activationCode := flag.String("activation-code", "", "one-time Controller Node activation code")
	activationURL := flag.String("activation-url", "https://api-clubpay.justix.uz", "ClubPay cloud API URL")
	nodeName := flag.String("node-name", "", "human-friendly Controller Node name")
	flag.Parse()
	if (*install && *uninstall) || (*setup && (*install || *uninstall)) {
		log.Fatal("--setup, --install and --uninstall cannot be combined")
	}
	if *uninstall {
		if err := uninstallWindowsControllerTask(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *setup {
		if err := setupControllerNode(*configPath, *activationCode, *activationURL, *nodeName); err != nil {
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
	if cfg.LocalDatabaseDataDir != "" {
		cfg.LocalDatabaseDataDir = resolvePathFromConfig(cfg.LocalDatabaseDataDir, *configPath)
	}
	if cfg.LocalDatabaseRuntimeDir != "" {
		cfg.LocalDatabaseRuntimeDir = resolvePathFromConfig(cfg.LocalDatabaseRuntimeDir, *configPath)
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
	var localDatabase *embeddedpostgres.EmbeddedPostgres
	if strings.EqualFold(cfg.LocalDatabaseMode, "embedded") {
		localDatabase, err = startEmbeddedDatabase(cfg)
		if err != nil {
			log.Fatalf("start local database: %v", err)
		}
		defer func() {
			if err := localDatabase.Stop(); err != nil {
				log.Printf("stop local database: %v", err)
			}
		}()
	}
	pool, err := connectDatabaseWithRetry(ctx, cfg.DatabaseURL, 90*time.Second)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, resolveMigrationsDir(*configPath)); err != nil {
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

type controllerEnrollment struct {
	NodeMode     string `json:"node_mode"`
	ClubID       string `json:"club_id"`
	NodeID       string `json:"node_id"`
	SyncToken    string `json:"sync_token"`
	CloudBaseURL string `json:"cloud_base_url"`
	CoreToken    string `json:"core_token"`
}

func setupControllerNode(configPath, activationCode, activationURL, nodeName string) error {
	activationCode = strings.TrimSpace(activationCode)
	if activationCode == "" {
		return errors.New("activation code is required: generate it in ClubPay web admin, then run setup again")
	}
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve controller config: %w", err)
	}
	if _, err := os.Stat(absoluteConfigPath); err == nil {
		return fmt.Errorf("%s already exists; this Controller is already configured (remove it only if you intentionally want to reinstall)", absoluteConfigPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect controller config: %w", err)
	}
	packageDir := filepath.Dir(absoluteConfigPath)
	if info, err := os.Stat(filepath.Join(packageDir, "migrations")); err != nil || !info.IsDir() {
		return errors.New("the Controller package is incomplete: migrations folder is missing")
	}
	if info, err := os.Stat(filepath.Join(packageDir, "web", "index.html")); err != nil || info.IsDir() {
		return errors.New("the Controller package is incomplete: web/index.html is missing")
	}
	hostname, _ := os.Hostname()
	nodeID := sanitizeLocalNodeID(hostname)
	if nodeID == "" {
		nodeID = "club-controller-" + randomLocalHex(3)
	}
	if strings.TrimSpace(nodeName) == "" {
		nodeName = hostname
	}
	cloudBaseURL := strings.TrimRight(strings.TrimSpace(activationURL), "/")
	if cloudBaseURL == "" {
		return errors.New("activation URL is required")
	}
	payload, _ := json.Marshal(map[string]string{
		"code":      activationCode,
		"node_id":   nodeID,
		"node_name": strings.TrimSpace(nodeName),
	})
	request, err := http.NewRequest(http.MethodPost, cloudBaseURL+"/api/controller/activate", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create activation request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("contact ClubPay cloud: %w", err)
	}
	defer response.Body.Close()
	var enrollment controllerEnrollment
	if err := json.NewDecoder(response.Body).Decode(&enrollment); err != nil {
		return fmt.Errorf("read activation response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("activation failed: %s", response.Status)
	}
	if enrollment.ClubID == "" || enrollment.SyncToken == "" || enrollment.CoreToken == "" || enrollment.NodeMode == "" {
		return errors.New("activation response is incomplete; contact ClubPay support")
	}
	if enrollment.CloudBaseURL != "" {
		cloudBaseURL = strings.TrimRight(enrollment.CloudBaseURL, "/")
	}
	publicBaseURL := detectLocalBaseURL(8080)
	databasePassword := randomLocalHex(24)
	configContents := strings.Join([]string{
		"# Generated by ClubPay Controller setup. Keep this file private.",
		"APP_ENV=production",
		"NODE_MODE=" + enrollment.NodeMode,
		"HTTP_ADDR=:8080",
		"WEB_DIR=web",
		"MIGRATIONS_DIR=migrations",
		"LOCAL_DATABASE_MODE=embedded",
		"LOCAL_DATABASE_DATA_DIR=data/postgres",
		"LOCAL_DATABASE_RUNTIME_DIR=runtime/postgres",
		"DATABASE_URL=postgres://clubpay:" + databasePassword + "@127.0.0.1:5432/clubpay?sslmode=disable",
		"PUBLIC_BASE_URL=" + publicBaseURL,
		"FRONTEND_BASE_URL=" + publicBaseURL,
		"CLOUD_BASE_URL=" + cloudBaseURL,
		"EDGE_SYNC_TOKEN=" + enrollment.SyncToken,
		"EDGE_NODE_ID=" + enrollment.NodeID,
		"EDGE_CLUB_ID=" + enrollment.ClubID,
		"CORE_MODE=ws",
		"CORE_TOKEN=" + enrollment.CoreToken,
		"CORE_TIMEOUT_MS=10000",
		"WOL_ENABLED=true",
		"WOL_BROADCAST_ADDR=255.255.255.255:9",
		"WOL_WAIT_SECONDS=60",
		"DEFAULT_PAYMENT_PROVIDER=click",
		"MOCK_PAYMENTS_ENABLED=false",
		"TELEGRAM_MINI_APP_ENABLED=false",
		"TELEGRAM_POLLING_ENABLED=false",
		"MANAGER_ONLINE_PAYMENTS_ENABLED=false",
		"EDGE_SYNC_INTERVAL_SECONDS=15",
		"VOUCHER_MIN_MINUTES=5",
		"VOUCHER_TTL_DAYS=30",
		"SESSION_GRACE_SECONDS=180",
		"",
	}, "\n")
	if err := os.WriteFile(absoluteConfigPath, []byte(configContents), 0o600); err != nil {
		return fmt.Errorf("save controller config: %w", err)
	}
	if err := envfile.LoadIntoEnvironment(absoluteConfigPath); err != nil {
		return fmt.Errorf("load generated config: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("read generated config: %w", err)
	}
	cfg.WebDir = resolvePathFromConfig(cfg.WebDir, absoluteConfigPath)
	cfg.LocalDatabaseDataDir = resolvePathFromConfig(cfg.LocalDatabaseDataDir, absoluteConfigPath)
	cfg.LocalDatabaseRuntimeDir = resolvePathFromConfig(cfg.LocalDatabaseRuntimeDir, absoluteConfigPath)
	localDatabase, err := startEmbeddedDatabase(cfg)
	if err != nil {
		return fmt.Errorf("prepare bundled local database: %w", err)
	}
	if err := localDatabase.Stop(); err != nil {
		return fmt.Errorf("finish local database setup: %w", err)
	}
	if runtime.GOOS == "windows" {
		if err := installWindowsControllerTask(absoluteConfigPath); err != nil {
			return err
		}
		result, err := exec.Command("schtasks.exe", "/Run", "/TN", "ClubPay Controller Node").CombinedOutput()
		if err != nil {
			return fmt.Errorf("start Windows Controller service: %w: %s", err, strings.TrimSpace(string(result)))
		}
	} else if runtime.GOOS == "linux" {
		if err := installLinuxControllerService(absoluteConfigPath); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("automatic service installation is not supported on %s", runtime.GOOS)
	}
	log.Printf("Controller Node is ready. Open %s/api/node/status", publicBaseURL)
	return nil
}

func startEmbeddedDatabase(cfg config.Config) (*embeddedpostgres.EmbeddedPostgres, error) {
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	password, hasPassword := parsed.User.Password()
	if parsed.User == nil || !hasPassword || parsed.User.Username() == "" || parsed.Hostname() == "" {
		return nil, errors.New("embedded database requires a PostgreSQL URL with user, password and host")
	}
	port := 5432
	if rawPort := parsed.Port(); rawPort != "" {
		port, err = strconv.Atoi(rawPort)
		if err != nil {
			return nil, fmt.Errorf("parse database port: %w", err)
		}
	}
	databaseName := strings.TrimPrefix(parsed.Path, "/")
	if databaseName == "" {
		return nil, errors.New("embedded database requires a database name")
	}
	if err := os.MkdirAll(cfg.LocalDatabaseDataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create local database data folder: %w", err)
	}
	if err := os.MkdirAll(cfg.LocalDatabaseRuntimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("create local database runtime folder: %w", err)
	}
	postgres := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username(parsed.User.Username()).
		Password(password).
		Database(databaseName).
		Port(uint32(port)).
		DataPath(cfg.LocalDatabaseDataDir).
		RuntimePath(cfg.LocalDatabaseRuntimeDir).
		CachePath(filepath.Join(filepath.Dir(cfg.LocalDatabaseRuntimeDir), "postgres-cache")).
		StartTimeout(90 * time.Second))
	if err := postgres.Start(); err != nil {
		return nil, err
	}
	return postgres, nil
}

func installLinuxControllerService(configPath string) error {
	if runtime.GOOS != "linux" {
		return errors.New("Linux service setup is available only on Linux")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	unit := fmt.Sprintf("[Unit]\nDescription=ClubPay Controller Node\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nExecStart=%q --config %q\nRestart=always\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n", executable, configPath)
	if err := os.WriteFile("/etc/systemd/system/clubpay-controller.service", []byte(unit), 0o644); err != nil {
		return fmt.Errorf("create systemd service (run setup with sudo): %w", err)
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", "--now", "clubpay-controller.service"}} {
		result, err := exec.Command("systemctl", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(result)))
		}
	}
	return nil
}

func detectLocalBaseURL(port int) string {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, _ := iface.Addrs()
			for _, address := range addresses {
				ip, _, parseErr := net.ParseCIDR(address.String())
				if parseErr == nil && ip.To4() != nil && !ip.IsLoopback() {
					return fmt.Sprintf("http://%s:%d", ip.String(), port)
				}
			}
		}
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func sanitizeLocalNodeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	previousDash := false
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			out.WriteRune(char)
			previousDash = false
		} else if !previousDash && out.Len() > 0 {
			out.WriteByte('-')
			previousDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func randomLocalHex(bytesLen int) string {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func resolveMigrationsDir(configPath string) string {
	if configured := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); configured != "" {
		return resolvePathFromConfig(configured, configPath)
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
