package payments

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestBuildPaymeSandboxCheckoutURLStabilizesPayload(t *testing.T) {
	checkoutURL, err := BuildPaymeSandboxCheckoutURL(
		"https://test.paycom.uz",
		"6a421930eaec1251d7724e09",
		"cp_02c1e464f1ac74044b662333",
		100000,
		"https://clubpay.justix.uz/payment/return?invoice_id=cp_02c1e464f1ac74044b662333",
	)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(checkoutURL, "https://test.paycom.uz/")
	if strings.Contains(token, "=") {
		t.Fatalf("sandbox token must not contain padding: %q", token)
	}
	unescaped, err := url.PathUnescape(token)
	if err != nil {
		t.Fatal(err)
	}
	payloadBytes, err := base64.RawStdEncoding.DecodeString(unescaped)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(payloadBytes)
	if len(payload)%3 != 0 {
		t.Fatalf("sandbox payload length must be divisible by 3, got %d: %s", len(payload), payload)
	}
	for _, want := range []string{
		"m=6a421930eaec1251d7724e09",
		"ac.order_id=cp_02c1e464f1ac74044b662333",
		"a=100000",
		"l=ru",
		"c=https://clubpay.justix.uz/payment/return?invoice_id=cp_02c1e464f1ac74044b662333",
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("payload does not contain %q: %s", want, payload)
		}
	}
}

func TestBuildPaymeSandboxCheckoutURLAddsFillerWhenNeeded(t *testing.T) {
	checkoutURL, err := BuildPaymeSandboxCheckoutURL(
		"https://test.paycom.uz",
		"6a421930eaec1251d7724e09",
		"cp_02c1e464f1ac74044b662333",
		100000,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(checkoutURL, "https://test.paycom.uz/")
	unescaped, err := url.PathUnescape(token)
	if err != nil {
		t.Fatal(err)
	}
	payloadBytes, err := base64.RawStdEncoding.DecodeString(unescaped)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(payloadBytes)
	if len(payload)%3 != 0 {
		t.Fatalf("sandbox payload length must be divisible by 3, got %d: %s", len(payload), payload)
	}
	if !strings.Contains(payload, ";x=") {
		t.Fatalf("payload should contain filler: %s", payload)
	}
}
