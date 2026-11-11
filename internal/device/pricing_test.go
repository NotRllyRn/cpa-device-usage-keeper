package device

import (
	"cpa-usage-keeper/internal/pricing"
	"testing"
)

func TestAggregatePricing(t *testing.T) {
	m := Model{Name: "example", Tokens: Tokens{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheWrite: 1_000_000}}
	p := pricing.ModelPricingConfig{Model: "example", PricingStyle: "openai", BasePrices: pricing.BasePrices{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}, ModelMultiplier: 2}
	got := estimate(m, &p)
	if got.Cost == nil || *got.Cost != 20 {
		t.Fatalf("%+v", got)
	}
	if estimate(m, nil).Cost != nil {
		t.Fatal("missing rate is not zero")
	}
	p.ModelMultiplier = 0
	if got := estimate(m, &p); got.Cost == nil || *got.Cost != 0 {
		t.Fatal(got)
	}
	p.Branches = []pricing.PriceBranch{{}}
	if estimate(m, &p).Cost != nil {
		t.Fatal("conditional daily price must be unknown")
	}
}
