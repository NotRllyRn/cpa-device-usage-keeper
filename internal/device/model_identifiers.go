package device

import (
	"context"
	"errors"
	"time"
)

// PricingModelIdentifiers reads identities only, not usage/cost breakdowns.
// SQLite bounds the distinct set and the context bounds the history scan.
func (s *Store) PricingModelIdentifiers(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT model FROM (
 SELECT model FROM device_prices
 UNION ALL
 SELECT json_extract(value, '$.model') AS model FROM snapshots, json_each(CAST(snapshots.payload AS TEXT), '$.models')
 ) LIMIT 10001`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := []string{}
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			return nil, err
		}
		models = append(models, model)
		if len(models) > 10000 {
			return nil, errors.New("too many model identifiers")
		}
	}
	return models, rows.Err()
}
