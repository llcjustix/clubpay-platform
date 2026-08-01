package core

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const defaultWOLBroadcastAddr = "255.255.255.255:9"

// SendWakeOnLAN sends a standard magic packet from the local Controller LAN.
// It must run on the club's Controller (for example Raspberry Pi), not cloud.
func SendWakeOnLAN(ctx context.Context, macAddress, broadcastAddr string) error {
	mac, err := net.ParseMAC(strings.TrimSpace(macAddress))
	if err != nil || len(mac) != 6 {
		return fmt.Errorf("wol_failed: invalid MAC address")
	}
	if strings.TrimSpace(broadcastAddr) == "" {
		broadcastAddr = defaultWOLBroadcastAddr
	}
	addr, err := net.ResolveUDPAddr("udp4", strings.TrimSpace(broadcastAddr))
	if err != nil {
		return fmt.Errorf("wol_failed: invalid broadcast address: %w", err)
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("wol_failed: open UDP socket: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetWriteDeadline(deadline)
	} else {
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	}
	packet := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		packet = append(packet, 0xff)
	}
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}
	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("wol_failed: send magic packet: %w", err)
	}
	return nil
}
