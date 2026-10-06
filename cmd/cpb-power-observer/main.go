// A bounded recovery observer: one configured VM, read-only Proxmox status API,
// TLS leaf pin, and an expiring PVE session ticket supplied on stdin only.
package main

import (
	"bufio"
	"clubpay/internal/bootpower"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "power_observer_failed")
		os.Exit(1)
	}
}
func run() error {
	base := os.Getenv("CPB_POWER_PVE_URL")
	u, e := url.Parse(base)
	pin, e2 := hex.DecodeString(os.Getenv("CPB_POWER_TLS_SHA256"))
	vmid, e3 := strconv.Atoi(os.Getenv("CPB_POWER_VMID"))
	node := os.Getenv("CPB_POWER_NODE")
	club := os.Getenv("CPB_POWER_CLUB_ID")
	pc := os.Getenv("CPB_POWER_PC_ID")
	token := os.Getenv("CPB_POWER_TOKEN")
	if e != nil || e2 != nil || e3 != nil || len(pin) != 32 || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(node) || vmid < 1 || club == "" || pc == "" || len(token) < 32 {
		return fmt.Errorf("configuration")
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("ticket_required")
	}
	ticket := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(ticket, "PVE:") {
		return fmt.Errorf("ticket_invalid")
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true, VerifyConnection: func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("certificate")
		}
		sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
		if subtle.ConstantTimeCompare(sum[:], pin) != 1 {
			return fmt.Errorf("certificate")
		}
		return nil
	}}}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /observe", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), []byte(token)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		var q bootpower.Request
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q) != nil || q.ClubID != club || q.ExternalPCID != pc || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(q.Nonce) {
			http.Error(w, "scope", 403)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(base, "/")+"/api2/json/nodes/"+node+"/qemu/"+strconv.Itoa(vmid)+"/status/current", nil)
		req.AddCookie(&http.Cookie{Name: "PVEAuthCookie", Value: ticket})
		res, e := client.Do(req)
		if e != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		defer res.Body.Close()
		var v struct {
			Data struct {
				VMID      int    `json:"vmid"`
				Status    string `json:"status"`
				QMPStatus string `json:"qmpstatus"`
			} `json:"data"`
		}
		if res.StatusCode != 200 || json.NewDecoder(http.MaxBytesReader(w, res.Body, 32768)).Decode(&v) != nil || v.Data.VMID != vmid || v.Data.Status != "stopped" || v.Data.QMPStatus != "stopped" {
			http.Error(w, "not_stopped", 409)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bootpower.Observation{Request: q, VMID: vmid, Status: "stopped", ObservedAt: time.Now().UTC()})
	})
	return (&http.Server{Addr: "127.0.0.1:18765", Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 8 * time.Second, WriteTimeout: 8 * time.Second}).ListenAndServe()
}
