package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWSControllerStartSessionCommand(t *testing.T) {
	controller := NewWSController("secret", time.Second)
	server := httptest.NewServer(http.HandlerFunc(controller.ServeHTTP))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?external_pc_id=pc-001&agent_token=secret", nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	commands := make(chan commandMessage, 1)
	go func() {
		var msg commandMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("read command: %v", err)
			return
		}
		commands <- msg
		err := conn.WriteJSON(commandResult{
			Type:      "command_result",
			CommandID: msg.CommandID,
			Status:    "ok",
			Payload: map[string]any{
				"external_pc_id":  "pc-001",
				"core_session_id": "core-session-1",
				"grant_id":        "grant-1",
				"started_at":      "2026-06-30T10:00:00Z",
				"ends_at":         "2026-06-30T11:00:00Z",
			},
		})
		if err != nil {
			t.Errorf("write result: %v", err)
		}
	}()

	result, err := controller.StartSession(context.Background(), StartSessionCommand{
		RequestID:       "cmd-1",
		GrantID:         "grant-1",
		PCExternalID:    "pc-001",
		DurationSeconds: 3600,
		Source:          "online_payment",
		PaymentOrderID:  "order-1",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if result.CoreSessionID != "core-session-1" {
		t.Fatalf("core session id = %q", result.CoreSessionID)
	}

	select {
	case cmd := <-commands:
		if cmd.Type != "command" || cmd.Name != "start_session" || cmd.CommandID != "cmd-1" {
			t.Fatalf("unexpected command: %#v", cmd)
		}
		if got := stringValue(cmd.Payload, "external_pc_id"); got != "pc-001" {
			t.Fatalf("external_pc_id = %q", got)
		}
		if got := intValue(cmd.Payload, "duration_seconds"); got != 3600 {
			t.Fatalf("duration_seconds = %d", got)
		}
	case <-time.After(time.Second):
		t.Fatal("command was not sent")
	}
}

func TestWSControllerWakesAndWaitsForAgentBeforeStartingSession(t *testing.T) {
	controller := NewWSController("secret", time.Second)
	server := httptest.NewServer(http.HandlerFunc(controller.ServeHTTP))
	defer server.Close()

	var agentConn *websocket.Conn
	controller.SetWakeHandler(func(ctx context.Context, externalPCID string) error {
		if externalPCID != "pc-001" {
			t.Fatalf("wake requested for %q", externalPCID)
		}
		conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?external_pc_id=pc-001&agent_token=secret", nil)
		if err != nil {
			return err
		}
		agentConn = conn
		go func() {
			var command commandMessage
			if err := conn.ReadJSON(&command); err != nil {
				t.Errorf("read start command: %v", err)
				return
			}
			if command.Name != "start_session" {
				t.Errorf("command name = %q", command.Name)
				return
			}
			_ = conn.WriteJSON(commandResult{
				Type: "command_result", CommandID: command.CommandID, Status: "ok",
				Payload: map[string]any{"core_session_id": "core-session-after-wake", "external_pc_id": "pc-001"},
			})
		}()
		return nil
	}, time.Second)
	defer func() {
		if agentConn != nil {
			_ = agentConn.Close()
		}
	}()

	result, err := controller.StartSession(context.Background(), StartSessionCommand{
		GrantID: "grant-1", PCExternalID: "pc-001", DurationSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("start after wake: %v", err)
	}
	if result.CoreSessionID != "core-session-after-wake" {
		t.Fatalf("core session id = %q", result.CoreSessionID)
	}
}

func TestWSControllerForwardsEvents(t *testing.T) {
	controller := NewWSController("secret", time.Second)
	events := make(chan EventMessage, 1)
	controller.SetEventHandler(func(ctx context.Context, event EventMessage) error {
		events <- event
		return nil
	})
	server := httptest.NewServer(http.HandlerFunc(controller.ServeHTTP))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?agent_token=secret", nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(wsInbound{
		Type:    "event",
		Name:    "pc_state_changed",
		EventID: "evt-1",
		Payload: map[string]any{
			"external_pc_id": "pc-001",
			"status":         "BUSY",
		},
	}); err != nil {
		t.Fatalf("write event: %v", err)
	}

	select {
	case event := <-events:
		if event.EventID != "evt-1" || event.Name != "pc_state_changed" || event.ExternalPCID != "pc-001" {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("event was not forwarded")
	}
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}
