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

func TestEdgeWOLRelayRoutesWakeToMatchingClub(t *testing.T) {
	relay := NewEdgeWOLRelay("edge-secret", time.Second)
	server := httptest.NewServer(http.HandlerFunc(relay.ServeHTTP))
	defer server.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer edge-secret")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?club_id=club-1&node_id=pi-1", header)
	if err != nil {
		t.Fatalf("connect relay: %v", err)
	}
	defer conn.Close()

	received := make(chan EdgeWOLCommand, 1)
	go func() {
		var command EdgeWOLCommand
		if err := conn.ReadJSON(&command); err != nil {
			t.Errorf("read command: %v", err)
			return
		}
		received <- command
		if err := conn.WriteJSON(edgeWOLResult{
			Type: "wake_result", CommandID: command.CommandID, Status: "ok",
		}); err != nil {
			t.Errorf("write result: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := relay.Wake(ctx, "club-1", "pc-1", "AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("Wake: %v", err)
	}
	select {
	case command := <-received:
		if command.Type != "wake" || command.ExternalPCID != "pc-1" || command.MACAddress != "AA:BB:CC:DD:EE:FF" {
			t.Fatalf("unexpected command: %#v", command)
		}
	case <-time.After(time.Second):
		t.Fatal("wake command was not sent")
	}
}

func TestEdgeWOLRelayRejectsWrongToken(t *testing.T) {
	relay := NewEdgeWOLRelay("edge-secret", time.Second)
	server := httptest.NewServer(http.HandlerFunc(relay.ServeHTTP))
	defer server.Close()

	_, response, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?club_id=club-1&node_id=pi-1&token=wrong", nil)
	if err == nil {
		t.Fatal("expected websocket authentication failure")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %#v, want 401", response)
	}
}

func TestEdgeWOLRelayRequiresOnlinePi(t *testing.T) {
	relay := NewEdgeWOLRelay("edge-secret", time.Second)
	err := relay.Wake(context.Background(), "club-1", "pc-1", "AA:BB:CC:DD:EE:FF")
	if err == nil || !strings.Contains(err.Error(), "no online Raspberry Pi relay") {
		t.Fatalf("Wake error = %v", err)
	}
}
