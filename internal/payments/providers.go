package payments

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	ProviderMock  = "mock"
	ProviderClick = "click"
	ProviderPayme = "payme"
)

func NormalizeProvider(value, fallback string) string {
	provider := strings.ToLower(strings.TrimSpace(value))
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(fallback))
	}
	switch provider {
	case ProviderClick, ProviderPayme, ProviderMock:
		return provider
	default:
		return ""
	}
}

func BuildPaymeCheckoutURL(baseURL, merchantID, orderID string, amountTiyin int64, returnURL string) (string, error) {
	if strings.TrimSpace(merchantID) == "" {
		return "", fmt.Errorf("Payme не настроен: нет merchant ID")
	}
	params := []string{
		"m=" + merchantID,
		"ac.order_id=" + orderID,
		"a=" + strconv.FormatInt(amountTiyin, 10),
		"l=ru",
	}
	if returnURL != "" {
		params = append(params, "c="+returnURL)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Join(params, ";")))
	return strings.TrimRight(baseURL, "/") + "/" + encoded, nil
}

func BuildClickCheckoutURL(baseURL, merchantID, serviceID, orderID string, amountTiyin int64, returnURL string) (string, error) {
	if strings.TrimSpace(merchantID) == "" {
		return "", fmt.Errorf("Click не настроен: нет merchant ID")
	}
	if strings.TrimSpace(serviceID) == "" {
		return "", fmt.Errorf("Click не настроен: нет service ID")
	}
	values := url.Values{}
	values.Set("merchant_id", merchantID)
	values.Set("service_id", serviceID)
	values.Set("transaction_param", orderID)
	values.Set("amount", formatClickAmount(amountTiyin))
	if returnURL != "" {
		values.Set("return_url", returnURL)
	}
	return strings.TrimRight(baseURL, "/") + "?" + values.Encode(), nil
}

func VerifyClickSign(secretKey, clickTransID, serviceID, merchantTransID, merchantPrepareID, amount, action, signTime, signString string) bool {
	if strings.TrimSpace(secretKey) == "" {
		return true
	}
	expected := ClickSign(secretKey, clickTransID, serviceID, merchantTransID, merchantPrepareID, amount, action, signTime)
	return strings.EqualFold(expected, strings.TrimSpace(signString))
}

func ClickSign(secretKey, clickTransID, serviceID, merchantTransID, merchantPrepareID, amount, action, signTime string) string {
	sum := md5.Sum([]byte(clickTransID + serviceID + secretKey + merchantTransID + merchantPrepareID + amount + action + signTime))
	return hex.EncodeToString(sum[:])
}

func formatClickAmount(amountTiyin int64) string {
	if amountTiyin%100 == 0 {
		return strconv.FormatInt(amountTiyin/100, 10)
	}
	return fmt.Sprintf("%.2f", float64(amountTiyin)/100)
}
