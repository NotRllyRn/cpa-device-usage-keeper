package device

import (
	"context"
	"cpa-usage-keeper/internal/pricing"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// ApplyBasePrices changes base rates atomically, retaining the current rules,
// branches, style and multipliers rather than trusting a stale browser config.
func (s *Store) ApplyBasePrices(ctx context.Context, request servicedto.PricingSyncApplyRequest) ([]pricing.ModelPricingConfig, error) {
	if request.Source != "models-dev" && request.Source != "litellm" {
		return nil, errors.New("invalid source")
	}
	if len(request.Items) == 0 || len(request.Items) > 1000 {
		return nil, errors.New("invalid item count")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	out := []pricing.ModelPricingConfig{}
	seen := map[string]bool{}
	for _, item := range request.Items {
		if !modelIdentifier.MatchString(item.Model) || seen[item.Model] {
			return nil, errors.New("invalid or duplicate model")
		}
		seen[item.Model] = true
		config := pricing.ModelPricingConfig{Model: item.Model, ModelMultiplier: 1, PricingStyle: item.PricingStyle, ConditionalMultipliers: []pricing.RuleConfig{}, Branches: []pricing.PriceBranch{}}
		var raw []byte
		err = tx.QueryRowContext(ctx, `SELECT payload FROM device_prices WHERE model=?`, item.Model).Scan(&raw)
		if err == nil {
			if err = json.Unmarshal(raw, &config); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		config.BasePrices = item.BasePrices
		if _, err = pricing.CompilePricingSnapshot([]pricing.ModelPricingConfig{config}, time.UTC); err != nil {
			return nil, err
		}
		raw, err = json.Marshal(config)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO device_prices(model,payload) VALUES(?,?) ON CONFLICT(model) DO UPDATE SET payload=excluded.payload`, config.Model, raw); err != nil {
			return nil, err
		}
		out = append(out, config)
	}
	return out, tx.Commit()
}
