package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// EdgeWOLRelay keeps an authenticated, outbound connection from one small
// device in each club LAN. Cloud never broadcasts packets itself: it asks the
// relay that belongs to the PC's club to do it.
type EdgeWOLRelay struct {
	token   string
	timeout time.Duration

	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[string]*edgeWOLClient
	pending map[string]chan edgeWOLResult
}

// EdgeWOLCommand is sent by Cloud to a Raspberry Pi. The Pi must not accept a
// MAC address from the public internet except through this authenticated socket.
type EdgeWOLCommand struct {
	Type         string `json:"type"`
	CommandID    string `json:"command_id"`
	ExternalPCID string `json:"external_pc_id"`
	MACAddress   string `json:"mac_address"`
}

type edgeWOLResult struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

type edgeWOLClient struct {
	relay  *EdgeWOLRelay
	conn   *websocket.Conn
	clubID string
	nodeID string
	send   chan any
	done   chan struct{}
}

func NewEdgeWOLRelay(token string, timeout time.Duration) *EdgeWOLRelay {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &EdgeWOLRelay{
		token:   strings.TrimSpace(token),
		timeout: timeout,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients: make(map[string]*edgeWOLClient),
		pending: make(map[string]chan edgeWOLResult),
	}
}

// ServeHTTP is intentionally separate from the Agent websocket. A Pi can only
// register one club, while an Agent can only register one gaming PC.
func (relay *EdgeWOLRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if relay.token == "" || edgeRelayToken(r) != relay.token {
		http.Error(w, "edge WoL token required", http.StatusUnauthorized)
		return
	}
	clubID := strings.TrimSpace(r.URL.Query().Get("club_id"))
	nodeID := strings.TrimSpace(r.URL.Query().Get("node_id"))
	if clubID == "" || nodeID == "" {
		http.Error(w, "club_id and node_id are required", http.StatusBadRequest)
		return
	}
	conn, err := relay.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &edgeWOLClient{
		relay: relay, conn: conn, clubID: clubID, nodeID: nodeID,
		send: make(chan any, 8), done: make(chan struct{}),
	}
	relay.register(client)
	defer relay.unregister(client)
	go client.writeLoop()
	client.readLoop()
}

func (relay *EdgeWOLRelay) Wake(ctx context.Context, clubID, externalPCID, macAddress string) error {
	clubID = strings.TrimSpace(clubID)
	externalPCID = strings.TrimSpace(externalPCID)
	macAddress = strings.TrimSpace(macAddress)
	if clubID == "" || externalPCID == "" || macAddress == "" {
		return fmt.Errorf("wol_failed: club, PC and MAC are required")
	}
	client := relay.clientForClub(clubID)
	if client == nil {
		return fmt.Errorf("wol_failed: no online Raspberry Pi relay for this club")
	}
	commandID := "edge_wake_" + safeCommandID(externalPCID) + "_" + unixMillis()
	result := make(chan edgeWOLResult, 1)
	relay.mu.Lock()
	relay.pending[commandID] = result
	relay.mu.Unlock()
	defer func() {
		relay.mu.Lock()
		delete(relay.pending, commandID)
		relay.mu.Unlock()
	}()

	command := EdgeWOLCommand{
		Type: "wake", CommandID: commandID, ExternalPCID: externalPCID, MACAddress: macAddress,
	}
	select {
	case client.send <- command:
	case <-client.done:
		return fmt.Errorf("wol_failed: Raspberry Pi relay disconnected")
	case <-ctx.Done():
		return ctx.Err()
	}

	timeout := relay.timeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case received := <-result:
		if strings.EqualFold(received.Status, "ok") {
			return nil
		}
		if strings.TrimSpace(received.Message) == "" {
			received.Message = "Raspberry Pi rejected wake command"
		}
		return fmt.Errorf("wol_failed: %s", received.Message)
	case <-timer.C:
		return fmt.Errorf("wol_failed: Raspberry Pi relay did not confirm wake")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (relay *EdgeWOLRelay) register(client *edgeWOLClient) {
	relay.mu.Lock()
	previous := relay.clients[client.clubID]
	relay.clients[client.clubID] = client
	relay.mu.Unlock()
	if previous != nil && previous != client {
		previous.close()
	}
}

func (relay *EdgeWOLRelay) unregister(client *edgeWOLClient) {
	client.close()
	relay.mu.Lock()
	if relay.clients[client.clubID] == client {
		delete(relay.clients, client.clubID)
	}
	relay.mu.Unlock()
}

func (relay *EdgeWOLRelay) clientForClub(clubID string) *edgeWOLClient {
	relay.mu.RLock()
	defer relay.mu.RUnlock()
	return relay.clients[strings.TrimSpace(clubID)]
}

func (relay *EdgeWOLRelay) complete(result edgeWOLResult) {
	relay.mu.RLock()
	pending := relay.pending[result.CommandID]
	relay.mu.RUnlock()
	if pending == nil {
		return
	}
	select {
	case pending <- result:
	default:
	}
}

func (client *edgeWOLClient) readLoop() {
	defer func() { _ = client.conn.Close() }()
	for {
		var result edgeWOLResult
		if err := client.conn.ReadJSON(&result); err != nil {
			return
		}
		if !strings.EqualFold(result.Type, "wake_result") || strings.TrimSpace(result.CommandID) == "" {
			continue
		}
		client.relay.complete(result)
	}
}

func (client *edgeWOLClient) writeLoop() {
	defer func() {
		client.close()
		_ = client.conn.Close()
	}()
	for {
		select {
		case message := <-client.send:
			if err := client.conn.WriteJSON(message); err != nil {
				return
			}
		case <-client.done:
			return
		}
	}
}

func (client *edgeWOLClient) close() {
	select {
	case <-client.done:
	default:
		close(client.done)
	}
}

func edgeRelayToken(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	if value := strings.TrimSpace(r.Header.Get("X-Edge-Token")); value != "" {
		return value
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}
