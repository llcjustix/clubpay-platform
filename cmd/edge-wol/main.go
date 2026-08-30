// edge-wol is the Raspberry Pi side of ClubPay Wake-on-LAN.
//
// It intentionally contains no payment, profile or database logic. Those stay
// in Cloud, so the existing Telegram Mini App and QR flow keep working. The Pi
// only holds an outbound authenticated websocket and sends a magic packet on
// the club LAN when Cloud needs to start a sleeping PC.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"clubpay/internal/core"

	"github.com/gorilla/websocket"
)

type config struct {
	WSURL         string
	Token         string
	NodeID        string
	ClubID        string
	BroadcastAddr string
}

type wakeResult struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to edge-wol.env")
	install := flag.Bool("install", false, "install the relay as a Windows startup task")
	uninstall := flag.Bool("uninstall", false, "remove the Windows startup task")
	flag.Parse()

	if *install && *uninstall {
		log.Fatal("use either --install or --uninstall")
	}
	if *uninstall {
		if err := uninstallWindowsTask(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *install {
		if _, err := loadConfig(*configPath); err != nil {
			log.Fatal(err)
		}
		if err := installWindowsTask(*configPath); err != nil {
			log.Fatal(err)
		}
		log.Printf("ClubPay Edge WoL will start automatically with Windows")
		return
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	delay := time.Second
	for ctx.Err() == nil {
		if err := run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("edge WoL connection ended: %v; reconnecting in %s", err, delay)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}

func defaultConfigPath() string {
	executable, err := os.Executable()
	if err != nil {
		return "edge-wol.env"
	}
	return filepath.Join(filepath.Dir(executable), "edge-wol.env")
}

func loadConfig(path string) (config, error) {
	values, err := readEnvFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config{}, fmt.Errorf("read relay config: %w", err)
	}
	cfg := config{
		WSURL:         configValue("EDGE_WOL_WS_URL", values),
		Token:         configValue("EDGE_WOL_TOKEN", values),
		NodeID:        configValue("EDGE_NODE_ID", values),
		ClubID:        configValue("EDGE_CLUB_ID", values),
		BroadcastAddr: configValue("WOL_BROADCAST_ADDR", values),
	}
	if cfg.BroadcastAddr == "" {
		cfg.BroadcastAddr = "255.255.255.255:9"
	}
	if cfg.WSURL == "" || cfg.Token == "" || cfg.NodeID == "" || cfg.ClubID == "" {
		return config{}, errors.New("EDGE_WOL_WS_URL, EDGE_WOL_TOKEN, EDGE_NODE_ID and EDGE_CLUB_ID are required")
	}
	if !strings.HasPrefix(cfg.WSURL, "wss://") {
		return config{}, errors.New("EDGE_WOL_WS_URL must use wss://")
	}
	return cfg, nil
}

func configValue(key string, values map[string]string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return strings.TrimSpace(values[key])
}

func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s:%d must be KEY=VALUE", path, lineNumber)
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "\"")
	}
	return values, scanner.Err()
}

func installWindowsTask(configPath string) error {
	if runtime.GOOS != "windows" {
		return errors.New("--install is available only on Windows; use systemd or Docker on Linux")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	command := fmt.Sprintf("\\\"%s\\\" --config \\\"%s\\\"", executable, configPath)
	result, err := exec.Command("schtasks.exe", "/Create", "/TN", "ClubPay Edge WoL", "/SC", "ONSTART", "/RU", "SYSTEM", "/RL", "HIGHEST", "/TR", command, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("create Windows startup task: %w: %s", err, strings.TrimSpace(string(result)))
	}
	return nil
}

func uninstallWindowsTask() error {
	if runtime.GOOS != "windows" {
		return errors.New("--uninstall is available only on Windows")
	}
	result, err := exec.Command("schtasks.exe", "/Delete", "/TN", "ClubPay Edge WoL", "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove Windows startup task: %w: %s", err, strings.TrimSpace(string(result)))
	}
	return nil
}

func run(ctx context.Context, cfg config) error {
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+cfg.Token)
	endpoint := cfg.WSURL + "?club_id=" + url.QueryEscape(cfg.ClubID) + "&node_id=" + url.QueryEscape(cfg.NodeID)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, header)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("connected to ClubPay Cloud as edge node %s for club %s", cfg.NodeID, cfg.ClubID)

	for {
		var command core.EdgeWOLCommand
		if err := conn.ReadJSON(&command); err != nil {
			return err
		}
		if command.Type != "wake" || command.CommandID == "" {
			continue
		}
		result := wakeResult{Type: "wake_result", CommandID: command.CommandID, Status: "ok"}
		wakeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := core.SendWakeOnLAN(wakeCtx, command.MACAddress, cfg.BroadcastAddr)
		cancel()
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			log.Printf("wake %s (%s) failed: %v", command.ExternalPCID, command.MACAddress, err)
		} else {
			log.Printf("magic packet sent to %s (%s)", command.ExternalPCID, command.MACAddress)
		}
		if err := conn.WriteJSON(result); err != nil {
			return err
		}
	}
}
