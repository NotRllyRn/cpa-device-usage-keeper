package device

import (
	"context"
	"cpa-usage-keeper/internal/pricing"
	dto "cpa-usage-keeper/internal/service/dto"
	"testing"
)

func TestSyncBasePricesAtomicPreservesConfig(t *testing.T) {
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	original := pricing.ModelPricingConfig{Model: "example", PricingStyle: "openai", ModelMultiplier: 2, ConditionalMultipliers: []pricing.RuleConfig{}, Branches: []pricing.PriceBranch{}, BasePrices: pricing.BasePrices{Input: 1}}
	if err := s.SavePrice(original); err != nil {
		t.Fatal(err)
	}
	item := dto.PricingSyncApplyItem{Model: "example", PricingStyle: "claude", BasePrices: pricing.BasePrices{Input: 3, Output: 4}}
	result, err := s.ApplyBasePrices(context.Background(), dto.PricingSyncApplyRequest{Source: "models-dev", Items: []dto.PricingSyncApplyItem{item}})
	if err != nil {
		t.Fatal(err)
	}
	if result[0].ModelMultiplier != 2 || result[0].PricingStyle != "openai" || result[0].BasePrices.Input != 3 {
		t.Fatal(result)
	}
	item.BasePrices.Input = 9
	bad := item
	bad.Model = "bad"
	bad.BasePrices.Input = -1
	if _, err = s.ApplyBasePrices(context.Background(), dto.PricingSyncApplyRequest{Source: "models-dev", Items: []dto.PricingSyncApplyItem{item, bad}}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	result, err = s.Prices()
	if err != nil || len(result) != 1 || result[0].BasePrices.Input != 3 {
		t.Fatal(result, err)
	}
	if _, err = s.ApplyBasePrices(context.Background(), dto.PricingSyncApplyRequest{Source: "models-dev", Items: []dto.PricingSyncApplyItem{item, item}}); err == nil {
		t.Fatal("duplicates accepted")
	}
}
