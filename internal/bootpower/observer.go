// Package bootpower provides an opt-in, independently authenticated power-state
// observer. It grants no storage or VM mutation capability.
package bootpower

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Request struct {
	ClubID       string `json:"club_id"`
	ExternalPCID string `json:"external_pc_id"`
	Nonce        string `json:"nonce"`
}
type Observation struct {
	Request
	VMID       int       `json:"vmid"`
	Status     string    `json:"status"`
	ObservedAt time.Time `json:"observed_at"`
}
type Config struct {
	URL          string `json:"url"`
	Token        string `json:"token"`
	ExternalPCID string `json:"external_pc_id"`
	VMID         int    `json:"vmid"`
}

func (c Config) Validate() error {
	u, e := url.Parse(c.URL)
	if e != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(c.Token) < 32 || c.ExternalPCID == "" || c.VMID < 1 {
		return fmt.Errorf("invalid_power_observer_config")
	}
	return nil
}
func (c Config) Stopped(ctx context.Context, club, pc string) error {
	if c.Validate() != nil || pc != c.ExternalPCID {
		return fmt.Errorf("power_observer_scope")
	}
	nonce := make([]byte, 32)
	if _, e := rand.Read(nonce); e != nil {
		return e
	}
	q := Request{club, pc, hex.EncodeToString(nonce)}
	data, _ := json.Marshal(q)
	req, e := http.NewRequestWithContext(ctx, "POST", c.URL, bytes.NewReader(data))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 7 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	started := time.Now().UTC()
	res, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("power_observer_unavailable")
	}
	defer res.Body.Close()
	var out Observation
	if res.StatusCode != 200 || json.NewDecoder(http.MaxBytesReader(nil, res.Body, 8192)).Decode(&out) != nil || out.Request != q || out.VMID != c.VMID || out.Status != "stopped" || out.ObservedAt.Before(started.Add(-time.Second)) || out.ObservedAt.After(time.Now().Add(time.Second)) {
		return fmt.Errorf("power_state_unverified")
	}
	return nil
}
