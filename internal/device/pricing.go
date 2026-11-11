package device

import (
	"encoding/json"
	"errors"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/helper"
	"cpa-usage-keeper/internal/pricing"
)

func (s *Store) Prices() ([]pricing.ModelPricingConfig, error) {
	rows, err := s.db.Query(`SELECT payload FROM device_prices ORDER BY model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pricing.ModelPricingConfig{}
	for rows.Next() {
		var raw []byte
		var p pricing.ModelPricingConfig
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SavePrice(p pricing.ModelPricingConfig) error {
	if !modelIdentifier.MatchString(p.Model) {
		return errors.New("invalid model")
	}
	if _, err := pricing.CompilePricingSnapshot([]pricing.ModelPricingConfig{p}, time.UTC); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO device_prices(model,payload) VALUES(?,?) ON CONFLICT(model) DO UPDATE SET payload=excluded.payload`, p.Model, raw)
	return err
}
func (s *Store) DeletePrice(model string) error {
	_, err := s.db.Exec(`DELETE FROM device_prices WHERE model=?`, model)
	return err
}

type Estimate struct {
	Cost   *float64 `json:"cost"`
	Status string   `json:"status"`
}

func estimate(m Model, p *pricing.ModelPricingConfig) Estimate {
	if p == nil {
		return Estimate{Status: "missing_price"}
	}
	if len(p.Branches) > 0 || len(p.ConditionalMultipliers) > 0 {
		return Estimate{Status: "requires_request_details"}
	}
	// ccusage categories are exclusive; Keeper's helper expects inclusive input.
	cost := helper.CalculateUsageTokenCost(helper.UsageTokenCostInput{InputTokens: m.Input + m.CacheRead + m.CacheWrite, OutputTokens: m.Output, CacheReadTokens: m.CacheRead, CacheCreationTokens: m.CacheWrite}, entities.ModelPriceSetting{PromptPricePer1M: p.BasePrices.Input, CompletionPricePer1M: p.BasePrices.Output, CacheReadPricePer1M: p.BasePrices.CacheRead, CacheWritePricePer1M: p.BasePrices.CacheWrite, PriceMultiplier: &p.ModelMultiplier})
	return Estimate{Cost: &cost, Status: "known"}
}
