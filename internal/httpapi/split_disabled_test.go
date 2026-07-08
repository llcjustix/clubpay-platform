package httpapi

import (
	"testing"

	"clubpay/internal/config"
	"clubpay/internal/payments"
)

func TestSplitDisabledKeepsFullAmountOnClub(t *testing.T) {
	s := &Server{cfg: config.Config{SplitPaymentsEnabled: false, PlatformFeeBPS: 200}}

	platformAmount, clubAmount := s.splitAmountsForClub(100_000, 500)
	if platformAmount != 0 || clubAmount != 100_000 {
		t.Fatalf("split disabled amounts = platform %d, club %d; want platform 0, club 100000", platformAmount, clubAmount)
	}

	payload := s.providerSplitPayload(payments.ProviderClick, splitSeed{
		ClickClubCntrgID:     "111",
		ClickPlatformCntrgID: "222",
		ClubAmountTiyin:      98_000,
		PlatformAmountTiyin:  2_000,
	})
	if len(payload) != 0 {
		t.Fatalf("split disabled provider payload len = %d, want 0", len(payload))
	}
}

func TestSplitDisabledOmitsProviderSpecificSplitFields(t *testing.T) {
	s := &Server{cfg: config.Config{SplitPaymentsEnabled: false, PlatformFeeBPS: 200}}
	order := providerOrder{
		ClickClubCntrgID:         "111",
		ClickPlatformCntrgID:     "222",
		PaymeClubReceiverID:      "club-receiver",
		PaymePlatformReceiverID:  "platform-receiver",
		SplitClubAmountTiyin:     98_000,
		SplitPlatformAmountTiyin: 2_000,
	}

	if split := s.clickSplitForOrder(order); len(split) != 0 {
		t.Fatalf("click split len = %d, want 0", len(split))
	}
	if receivers := s.paymeReceiversForOrder(order); len(receivers) != 0 {
		t.Fatalf("payme receivers len = %d, want 0", len(receivers))
	}
}
