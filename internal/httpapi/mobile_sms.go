package httpapi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var mobileSMSEndpoint = "https://routee.sayqal.uz/sms/TransmitSMS"

var mobileSMSHTTPClient = &http.Client{Timeout: 8 * time.Second}

// smsAccessToken follows Sayqal's documented TransmitSMS signing formula.
// The secret is never persisted, returned, or logged.
func smsAccessToken(username, secret string, unixTime int64) string {
	sum := md5.Sum([]byte(fmt.Sprintf("TransmitSMS %s %s %d", username, secret, unixTime)))
	return hex.EncodeToString(sum[:])
}

func (s *Server) sendMobileOTP(ctx context.Context, phone, code string) error {
	username := strings.TrimSpace(s.cfg.SMSUsername)
	secret := strings.TrimSpace(s.cfg.SMSSecretKey)
	if username == "" || secret == "" || s.cfg.SMSService <= 0 {
		return fmt.Errorf("sms otp is not configured")
	}
	now := time.Now().Unix()
	body := map[string]any{
		"utime":    now,
		"username": username,
		"service":  map[string]any{"service": s.cfg.SMSService},
		"message": map[string]any{
			"smsid": fmt.Sprintf("clubpay-login-%d", now),
			"phone": strings.TrimPrefix(phone, "+"),
			"text":  "ClubPay: код входа " + code + ". Никому не сообщайте код.",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mobileSMSEndpoint, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Access-Token", smsAccessToken(username, secret, now))
	response, err := mobileSMSHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sms provider unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("sms provider rejected request")
	}
	var result struct {
		TransactionID any `json:"transactionid"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("sms provider invalid response")
	}
	if result.TransactionID == nil {
		return fmt.Errorf("sms provider did not accept message")
	}
	return nil
}
