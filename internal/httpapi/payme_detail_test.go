package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestPaymeDetailBase64(t *testing.T) {
	payload := []byte(`{
		"items": [{
			"name": "Computer time",
			"mxik": "06401004002000000",
			"package_code": "1506113",
			"unit_code": "796",
			"price_tiyin": 800000,
			"qty": 1,
			"vat_percent": 0
		}]
	}`)

	encoded := paymeDetailBase64(payload)
	if encoded == "" {
		t.Fatal("expected Payme detail payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode detail: %v", err)
	}

	var detail struct {
		ReceiptType int `json:"receipt_type"`
		Items       []struct {
			Title       string `json:"title"`
			Price       int64  `json:"price"`
			Count       int64  `json:"count"`
			Code        string `json:"code"`
			PackageCode string `json:"package_code"`
			Units       int64  `json:"units"`
			VATPercent  int64  `json:"vat_percent"`
		} `json:"items"`
	}
	if err := json.Unmarshal(decoded, &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if detail.ReceiptType != 0 {
		t.Fatalf("receipt_type = %d, want 0", detail.ReceiptType)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(detail.Items))
	}
	item := detail.Items[0]
	if item.Title != "Computer time" || item.Price != 800000 || item.Count != 1 || item.Code != "06401004002000000" || item.PackageCode != "1506113" || item.Units != 796 || item.VATPercent != 0 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestPaymeDetailBase64AllowsStaticCashierData(t *testing.T) {
	if encoded := paymeDetailBase64([]byte(`{}`)); encoded != "" {
		t.Fatalf("expected empty detail, got %q", encoded)
	}
}
