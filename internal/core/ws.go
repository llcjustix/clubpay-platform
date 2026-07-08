package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type EventMessage struct {
	Type          string         `json:"type"`
	Name          string         `json:"name"`
	EventID       string         `json:"event_id"`
	TS            string         `json:"ts"`
	ClubID        string         `json:"club_id,omitempty"`
	ExternalPCID  string         `json:"external_pc_id,omitempty"`
	CoreSessionID string         `json:"core_session_id,omitempty"`
	GrantID       string         `json:"grant_id,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type EventHandler func(context.Context, EventMessage) error

type WSController struct {
	token   string
	timeout time.Duration

	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[string]*wsClient
	pending map[string]chan commandResult
	events  EventHandler
}

type wsClient struct {
	controller   *WSController
	conn         *websocket.Conn
	externalPCID string
	send         chan any
	done         chan struct{}
}

type commandMessage struct {
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	CommandID string         `json:"command_id"`
	TS        string         `json:"ts"`
	Payload   map[string]any `json:"payload"`
}

type commandResult struct {
	Type      string         `json:"type"`
	CommandID string         `json:"command_id"`
	Status    string         `json:"status"`
	ErrorCode string         `json:"error_code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type wsInbound struct {
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	CommandID string         `json:"command_id"`
	EventID   string         `json:"event_id"`
	TS        string         `json:"ts"`
	Status    string         `json:"status"`
	ErrorCode string         `json:"error_code"`
	Message   string         `json:"message"`
	Payload   map[string]any `json:"payload"`
}

func NewWSController(token string, timeout time.Duration) *WSController {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &WSController{
		token:   strings.TrimSpace(token),
		timeout: timeout,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients: make(map[string]*wsClient),
		pending: make(map[string]chan commandResult),
	}
}

func (c *WSController) SetEventHandler(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = handler
}

func (c *WSController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.token != "" && tokenFromRequest(r) != c.token {
		http.Error(w, "core token required", http.StatusUnauthorized)
		return
	}

	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{
		controller:   c,
		conn:         conn,
		externalPCID: strings.TrimSpace(firstNonEmpty(r.URL.Query().Get("external_pc_id"), r.URL.Query().Get("pc_id"))),
		send:         make(chan any, 16),
		done:         make(chan struct{}),
	}
	if client.externalPCID != "" {
		c.registerClient(client.externalPCID, client)
	}

	go client.writeLoop()
	client.readLoop()
}

func (c *WSController) GetPCStatus(ctx context.Context, externalPCID string) (PCStatus, error) {
	result, err := c.sendCommand(ctx, externalPCID, "get_status", "status_"+safeCommandID(externalPCID)+"_"+unixMillis(), map[string]any{
		"external_pc_id": externalPCID,
	})
	if err != nil {
		if errors.Is(err, ErrAgentOffline) {
			return PCStatus{ExternalPCID: externalPCID, Status: "offline", AgentOnline: false, ControllerOnline: true}, nil
		}
		return PCStatus{}, err
	}
	payload := result.Payload
	status := normalizePCState(stringValue(payload, "pc_state", "status"))
	if status == "" {
		status = "unknown"
	}
	lastSeen := time.Now().UTC()
	return PCStatus{
		ExternalPCID:     firstNonEmpty(stringValue(payload, "external_pc_id"), externalPCID),
		Status:           status,
		CurrentSessionID: stringValue(payload, "core_session_id", "current_session_id"),
		CurrentGrantID:   stringValue(payload, "grant_id", "current_grant_id"),
		RemainingSeconds: intValue(payload, "remaining_seconds"),
		AgentOnline:      true,
		ControllerOnline: true,
		LastSeenAt:       &lastSeen,
	}, nil
}

func (c *WSController) StartSession(ctx context.Context, cmd StartSessionCommand) (StartSessionResult, error) {
	if strings.TrimSpace(cmd.PCExternalID) == "" {
		return StartSessionResult{}, fmt.Errorf("external_pc_id is required")
	}
	duration := cmd.DurationSeconds
	if duration <= 0 {
		duration = cmd.DurationMinutes * 60
	}
	now := time.Now().UTC()
	endsAt := now.Add(time.Duration(duration) * time.Second)
	result, err := c.sendCommand(ctx, cmd.PCExternalID, "start_session", defaultString(cmd.RequestID, "start_"+cmd.GrantID), map[string]any{
		"external_pc_id":   cmd.PCExternalID,
		"grant_id":         cmd.GrantID,
		"payment_order_id": cmd.PaymentOrderID,
		"granted_seconds":  duration,
		"duration_seconds": duration,
		"duration_minutes": cmd.DurationMinutes,
		"ends_at":          endsAt.Format(time.RFC3339),
		"start_at":         now.Format(time.RFC3339),
		"source":           cmd.Source,
		"invoice_id":       cmd.InvoiceID,
		"extend_url":       cmd.ExtendURL,
	})
	if err != nil {
		return StartSessionResult{}, err
	}
	payload := result.Payload
	startedAt := timeValue(payload, "started_at", "start_at")
	if startedAt == nil {
		startedAt = &now
	}
	resultEndsAt := timeValue(payload, "ends_at", "planned_ends_at")
	if resultEndsAt == nil {
		resultEndsAt = &endsAt
	}
	return StartSessionResult{
		Status:        "accepted",
		CoreSessionID: stringValue(payload, "core_session_id", "session_id"),
		ExternalPCID:  firstNonEmpty(stringValue(payload, "external_pc_id"), cmd.PCExternalID),
		GrantID:       firstNonEmpty(stringValue(payload, "grant_id"), cmd.GrantID),
		StartedAt:     startedAt,
		EndsAt:        resultEndsAt,
	}, nil
}

func (c *WSController) ExtendSession(ctx context.Context, coreSessionID string, cmd ExtendSessionCommand) (ExtendSessionResult, error) {
	if coreSessionID == "" {
		return ExtendSessionResult{}, fmt.Errorf("core_session_id is required")
	}
	result, err := c.sendCommand(ctx, cmd.ExternalPCID, "extend_session", defaultString(cmd.RequestID, "extend_"+cmd.GrantID), map[string]any{
		"core_session_id":  coreSessionID,
		"external_pc_id":   cmd.ExternalPCID,
		"grant_id":         cmd.GrantID,
		"payment_order_id": cmd.PaymentOrderID,
		"added_seconds":    cmd.AddSeconds,
		"add_seconds":      cmd.AddSeconds,
		"source":           cmd.Source,
		"invoice_id":       cmd.InvoiceID,
	})
	if err != nil {
		return ExtendSessionResult{}, err
	}
	payload := result.Payload
	return ExtendSessionResult{
		Status:           "accepted",
		CoreSessionID:    firstNonEmpty(stringValue(payload, "core_session_id"), coreSessionID),
		GrantID:          firstNonEmpty(stringValue(payload, "grant_id"), cmd.GrantID),
		OldEndsAt:        timeValue(payload, "old_ends_at"),
		NewEndsAt:        timeValue(payload, "new_ends_at", "ends_at", "planned_ends_at"),
		RemainingSeconds: intValue(payload, "remaining_seconds"),
	}, nil
}

func (c *WSController) EndSession(ctx context.Context, coreSessionID string, cmd EndSessionCommand) (EndSessionResult, error) {
	if coreSessionID == "" {
		return EndSessionResult{}, fmt.Errorf("core_session_id is required")
	}
	commandPayload := map[string]any{
		"core_session_id": coreSessionID,
		"external_pc_id":  cmd.ExternalPCID,
		"reason":          cmd.Reason,
		"ended_by":        cmd.EndedBy,
	}
	result, err := c.sendCommandToSession(ctx, coreSessionID, "end_session", defaultString(cmd.RequestID, "end_"+coreSessionID), commandPayload)
	if err != nil {
		return EndSessionResult{}, err
	}
	payload := result.Payload
	return EndSessionResult{
		Status:           "ended",
		CoreSessionID:    firstNonEmpty(stringValue(payload, "core_session_id"), coreSessionID),
		ExternalPCID:     stringValue(payload, "external_pc_id"),
		StartedAt:        timeValue(payload, "started_at"),
		EndedAt:          timeValue(payload, "ended_at"),
		PlannedEndsAt:    timeValue(payload, "planned_ends_at", "ends_at"),
		RemainingSeconds: intValue(payload, "remaining_seconds"),
	}, nil
}

func (c *WSController) Lock(ctx context.Context, externalPCID, reason string) error {
	_, err := c.sendCommand(ctx, externalPCID, "lock", "lock_"+safeCommandID(externalPCID)+"_"+unixMillis(), map[string]any{
		"external_pc_id": externalPCID,
		"reason":         reason,
	})
	return err
}

func (c *WSController) Unlock(ctx context.Context, externalPCID, reason string) error {
	_, err := c.sendCommand(ctx, externalPCID, "unlock", "unlock_"+safeCommandID(externalPCID)+"_"+unixMillis(), map[string]any{
		"external_pc_id": externalPCID,
		"reason":         reason,
	})
	return err
}

func (c *WSController) Wake(ctx context.Context, externalPCID string) error {
	_, err := c.sendCommand(ctx, externalPCID, "wake", "wake_"+safeCommandID(externalPCID)+"_"+unixMillis(), map[string]any{
		"external_pc_id": externalPCID,
	})
	return err
}

func (c *WSController) Sleep(ctx context.Context, externalPCID string) error {
	_, err := c.sendCommand(ctx, externalPCID, "sleep", "sleep_"+safeCommandID(externalPCID)+"_"+unixMillis(), map[string]any{
		"external_pc_id": externalPCID,
	})
	return err
}

func (c *WSController) SetRepair(ctx context.Context, externalPCID string, on bool) error {
	_, err := c.sendCommand(ctx, externalPCID, "set_repair", "repair_"+safeCommandID(externalPCID)+"_"+unixMillis(), map[string]any{
		"external_pc_id": externalPCID,
		"on":             on,
	})
	return err
}

var ErrAgentOffline = errors.New("agent_offline")

func (c *WSController) sendCommand(ctx context.Context, externalPCID, name, commandID string, payload map[string]any) (commandResult, error) {
	externalPCID = strings.TrimSpace(externalPCID)
	if externalPCID == "" {
		return commandResult{}, fmt.Errorf("external_pc_id is required")
	}
	client := c.clientForPC(externalPCID)
	if client == nil {
		return commandResult{}, ErrAgentOffline
	}
	return c.sendCommandToClient(ctx, client, name, commandID, payload)
}

func (c *WSController) sendCommandToSession(ctx context.Context, coreSessionID, name, commandID string, payload map[string]any) (commandResult, error) {
	if externalPCID := stringValue(payload, "external_pc_id"); externalPCID != "" {
		return c.sendCommand(ctx, externalPCID, name, commandID, payload)
	}
	c.mu.RLock()
	var chosen *wsClient
	for _, client := range c.clients {
		chosen = client
		break
	}
	c.mu.RUnlock()
	if chosen == nil {
		return commandResult{}, ErrAgentOffline
	}
	return c.sendCommandToClient(ctx, chosen, name, commandID, payload)
}

func (c *WSController) sendCommandToClient(ctx context.Context, client *wsClient, name, commandID string, payload map[string]any) (commandResult, error) {
	if commandID == "" {
		commandID = name + "_" + unixMillis()
	}
	resultCh := make(chan commandResult, 1)
	c.mu.Lock()
	c.pending[commandID] = resultCh
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, commandID)
		c.mu.Unlock()
	}()

	msg := commandMessage{
		Type:      "command",
		Name:      name,
		CommandID: commandID,
		TS:        time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
	select {
	case client.send <- msg:
	case <-ctx.Done():
		return commandResult{}, ctx.Err()
	case <-client.done:
		return commandResult{}, ErrAgentOffline
	}

	timeout := time.NewTimer(c.timeout)
	defer timeout.Stop()
	select {
	case result := <-resultCh:
		if !strings.EqualFold(result.Status, "ok") && !strings.EqualFold(result.Status, "accepted") {
			code := defaultString(result.ErrorCode, "internal_error")
			message := defaultString(result.Message, code)
			return result, fmt.Errorf("%s: %s", code, message)
		}
		return result, nil
	case <-timeout.C:
		return commandResult{}, fmt.Errorf("command_timeout: %s", commandID)
	case <-ctx.Done():
		return commandResult{}, ctx.Err()
	case <-client.done:
		return commandResult{}, ErrAgentOffline
	}
}

func (c *WSController) clientForPC(externalPCID string) *wsClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clients[externalPCID]
}

func (c *WSController) registerClient(externalPCID string, client *wsClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if previous := c.clients[externalPCID]; previous != nil && previous != client {
		previous.close()
	}
	client.externalPCID = externalPCID
	c.clients[externalPCID] = client
}

func (c *WSController) unregisterClient(client *wsClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if client.externalPCID != "" && c.clients[client.externalPCID] == client {
		delete(c.clients, client.externalPCID)
	}
}

func (client *wsClient) readLoop() {
	defer func() {
		client.controller.unregisterClient(client)
		client.close()
		_ = client.conn.Close()
	}()
	for {
		var msg wsInbound
		if err := client.conn.ReadJSON(&msg); err != nil {
			return
		}
		if msg.Payload == nil {
			msg.Payload = map[string]any{}
		}
		externalPCID := firstNonEmpty(msg.ExternalPCID(), stringValue(msg.Payload, "external_pc_id"))
		if externalPCID != "" && client.externalPCID == "" {
			client.controller.registerClient(externalPCID, client)
		}
		switch msg.Type {
		case "command_result":
			client.controller.completeCommand(commandResult{
				Type:      msg.Type,
				CommandID: msg.CommandID,
				Status:    msg.Status,
				ErrorCode: msg.ErrorCode,
				Message:   msg.Message,
				Payload:   msg.Payload,
			})
		case "event":
			client.controller.handleEvent(EventMessage{
				Type:          msg.Type,
				Name:          msg.Name,
				EventID:       msg.EventID,
				TS:            msg.TS,
				ExternalPCID:  externalPCID,
				CoreSessionID: firstNonEmpty(stringValue(msg.Payload, "core_session_id"), stringValue(msg.Payload, "session_id")),
				GrantID:       stringValue(msg.Payload, "grant_id"),
				ClubID:        stringValue(msg.Payload, "club_id"),
				Payload:       msg.Payload,
			})
		}
	}
}

func (client *wsClient) writeLoop() {
	defer func() {
		client.close()
		_ = client.conn.Close()
	}()
	for {
		select {
		case msg := <-client.send:
			if err := client.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-client.done:
			return
		}
	}
}

func (client *wsClient) close() {
	select {
	case <-client.done:
	default:
		close(client.done)
	}
}

func (c *WSController) completeCommand(result commandResult) {
	c.mu.RLock()
	ch := c.pending[result.CommandID]
	c.mu.RUnlock()
	if ch == nil {
		return
	}
	select {
	case ch <- result:
	default:
	}
}

func (c *WSController) handleEvent(event EventMessage) {
	c.mu.RLock()
	handler := c.events
	c.mu.RUnlock()
	if handler == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = handler(ctx, event)
}

func (msg wsInbound) ExternalPCID() string {
	return firstNonEmpty(stringValue(msg.Payload, "external_pc_id"), stringValue(msg.Payload, "pc_external_id"))
}

func tokenFromRequest(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	if value := strings.TrimSpace(r.Header.Get("X-Agent-Token")); value != "" {
		return value
	}
	if value := strings.TrimSpace(r.URL.Query().Get("agent_token")); value != "" {
		return value
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func normalizePCState(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "FREE", "AVAILABLE":
		return "available"
	case "OCCUPIED", "BUSY":
		return "occupied"
	case "FROZEN", "BLOCKED", "LOCKED":
		return "blocked"
	case "SLEEPING", "SLEEP":
		return "sleeping"
	case "REPAIR", "MAINTENANCE":
		return "maintenance"
	case "OFFLINE":
		return "offline"
	case "ATTENTION", "UNKNOWN":
		return "unknown"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func stringValue(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case fmt.Stringer:
			if strings.TrimSpace(typed.String()) != "" {
				return strings.TrimSpace(typed.String())
			}
		}
	}
	return ""
}

func intValue(payload map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int64:
			return int(typed)
		case float64:
			return int(typed)
		case string:
			var result int
			if _, err := fmt.Sscanf(typed, "%d", &result); err == nil {
				return result
			}
		}
	}
	return 0
}

func timeValue(payload map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value := stringValue(payload, key)
		if value == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return &parsed
		}
	}
	return nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func safeCommandID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_")
	return replacer.Replace(value)
}

func unixMillis() string {
	return fmt.Sprintf("%d", time.Now().UTC().UnixMilli())
}
