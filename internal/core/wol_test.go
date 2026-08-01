package core

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSendWakeOnLANSendsMagicPacket(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := SendWakeOnLAN(ctx, "AA:BB:CC:DD:EE:FF", listener.LocalAddr().String()); err != nil {
		t.Fatalf("send wake: %v", err)
	}
	buf := make([]byte, 128)
	_ = listener.SetReadDeadline(time.Now().Add(time.Second))
	n, _, err := listener.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read magic packet: %v", err)
	}
	if n != 102 {
		t.Fatalf("magic packet length = %d, want 102", n)
	}
	for i := 0; i < 6; i++ {
		if buf[i] != 0xff {
			t.Fatalf("header byte %d = %x", i, buf[i])
		}
	}
	if got := buf[6:12]; string(got) != string([]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}) {
		t.Fatalf("first MAC block = %x", got)
	}
}
