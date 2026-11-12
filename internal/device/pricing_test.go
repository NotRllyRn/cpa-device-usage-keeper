package device

import (
	"cpa-usage-keeper/internal/pricing"
	"testing"
)

func TestPiPriceFallback(t *testing.T) {
	base := &pricing.ModelPricingConfig{Model: "gpt-example", ModelMultiplier: 1, BasePrices: pricing.BasePrices{Input: 2}}
	prices := map[string]*pricing.ModelPricingConfig{"gpt-example": base}
	if resolvePrice("[pi] gpt-example", prices) != base {
		t.Fatal("Pi prefix did not resolve")
	}
	for _, name := range []string{"[other] gpt-example", "[pi] gpt-example-mini", "[pi] provider/gpt-example", "gpt-example-new"} {
		if resolvePrice(name, prices) != nil {
			t.Fatalf("unexpected fuzzy match: %s", name)
		}
	}
	m := Model{Name: "[pi] gpt-example", Tokens: Tokens{Input: 1000000}}
	if got := estimate(m, resolvePrice(m.Name, prices)); got.Cost == nil || *got.Cost != 2 {
		t.Fatal(got)
	}
	override := &pricing.ModelPricingConfig{Model: m.Name}
	prices[m.Name] = override
	if resolvePrice(m.Name, prices) != override {
		t.Fatal("explicit override lost")
	}
}

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
