package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"clubpay/internal/config"
)

func TestSMSAccessTokenMatchesSayqalFormula(t *testing.T) {
	if got, want := smsAccessToken("justix", "secret", 1710160313), "385bded9498e879635af2a665c9a121a"; got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
}

func TestSendMobileOTPUsesSayqalRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Access-Token") == "" {
			t.Fatal("invalid Sayqal request")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) == "" || string(body) == "{}" {
			t.Fatal("SMS body is empty")
		}
		_, _ = w.Write([]byte(`{"transactionid":"42","smsid":"test","parts":1}`))
	}))
	defer server.Close()
	previousEndpoint, previousClient := mobileSMSEndpoint, mobileSMSHTTPClient
	mobileSMSEndpoint, mobileSMSHTTPClient = server.URL, server.Client()
	defer func() { mobileSMSEndpoint, mobileSMSHTTPClient = previousEndpoint, previousClient }()
	s := &Server{cfg: config.Config{SMSUsername: "test", SMSSecretKey: "secret", SMSService: 1}}
	if err := s.sendMobileOTP(context.Background(), "+998901234567", "123456"); err != nil {
		t.Fatal(err)
	}
}
