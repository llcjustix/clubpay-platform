// edge-wol is the Raspberry Pi side of ClubPay Wake-on-LAN.
//
// It intentionally contains no payment, profile or database logic. Those stay
// in Cloud, so the existing Telegram Mini App and QR flow keep working. The Pi
// only holds an outbound authenticated websocket and sends a magic packet on
// the club LAN when Cloud needs to start a sleeping PC.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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
	cfg, err := loadConfig()
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

func loadConfig() (config, error) {
	cfg := config{
		WSURL:         strings.TrimSpace(os.Getenv("EDGE_WOL_WS_URL")),
		Token:         strings.TrimSpace(os.Getenv("EDGE_WOL_TOKEN")),
		NodeID:        strings.TrimSpace(os.Getenv("EDGE_NODE_ID")),
		ClubID:        strings.TrimSpace(os.Getenv("EDGE_CLUB_ID")),
		BroadcastAddr: strings.TrimSpace(os.Getenv("WOL_BROADCAST_ADDR")),
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
