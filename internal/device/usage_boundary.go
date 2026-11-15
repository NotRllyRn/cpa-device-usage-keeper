package device

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// EarliestUsageDay uses the same model selection as Overview, independently of
// its date range and pricing. Unfiltered reads retain the cheap SQL boundary.
func (s *Store) EarliestUsageDay(ctx context.Context, deviceID, agent, family string) (*string, error) {
	if !validModelFamily(family) {
		return nil, errors.New("invalid model family")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	columns := "MIN(day)"
	if family != "" {
		columns = "day,payload"
	}
	query := `SELECT ` + columns + ` FROM snapshots WHERE 1=1`
	args := []any{}
	if deviceID != "" {
		query += ` AND device_id=?`
		args = append(args, deviceID)
	}
	if agent != "" {
		query += ` AND agent=?`
		args = append(args, agent)
	}
	if family != "" {
		// Model names and the unattributed remainder live in the upload payload.
		// Bound the scan just like aggregation; never return a partial boundary.
		query += ` ORDER BY day LIMIT 100001`
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var scanned int
		var payloadBytes int64
		for rows.Next() {
			scanned++
			var day string
			var raw []byte
			if err := rows.Scan(&day, &raw); err != nil {
				return nil, err
			}
			payloadBytes += int64(len(raw))
			if scanned > 100000 || payloadBytes > 256<<20 {
				return nil, errors.New("usage boundary exceeds scan limits; narrow the filters")
			}
			var snapshot Snapshot
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				return nil, err
			}
			if _, _, matches := selectModelFamily(snapshot, family); matches {
				return &day, nil
			}
		}
		return nil, rows.Err()
	}
	var day sql.NullString
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&day); err != nil {
		return nil, err
	}
	if !day.Valid {
		return nil, nil
	}
	return &day.String, nil
}
