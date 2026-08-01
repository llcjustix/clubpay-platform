package core

import (
	"context"
	"testing"
)

func TestMockAdapterWakesSleepingPCBeforeStartingSession(t *testing.T) {
	mock := NewMockAdapter()
	if err := mock.Sleep(context.Background(), "pc-001"); err != nil {
		t.Fatalf("sleep: %v", err)
	}

	before, err := mock.GetPCStatus(context.Background(), "pc-001")
	if err != nil {
		t.Fatalf("status before start: %v", err)
	}
	if before.Status != "sleeping" || before.AgentOnline {
		t.Fatalf("expected sleeping offline Agent, got %#v", before)
	}

	result, err := mock.StartSession(context.Background(), StartSessionCommand{
		GrantID: "grant-1", PCExternalID: "pc-001", DurationSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if result.Status != "accepted" {
		t.Fatalf("start status = %q", result.Status)
	}

	after, err := mock.GetPCStatus(context.Background(), "pc-001")
	if err != nil {
		t.Fatalf("status after start: %v", err)
	}
	if after.Status != "available" || !after.AgentOnline || mock.WakeCount("pc-001") != 1 {
		t.Fatalf("mock did not complete wake/reconnect flow: %#v wakes=%d", after, mock.WakeCount("pc-001"))
	}
}
