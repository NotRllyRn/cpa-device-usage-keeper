package device

import (
	"context"
	"database/sql"
	"time"
)

// EarliestUsageDay reads only the boundary, never snapshot payloads. The same
// device/agent predicates as Overview apply, independently of its date range.
func (s *Store) EarliestUsageDay(ctx context.Context, deviceID, agent string) (*string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := `SELECT MIN(day) FROM snapshots WHERE 1=1`
	args := []any{}
	if deviceID != "" {
		query += ` AND device_id=?`
		args = append(args, deviceID)
	}
	if agent != "" {
		query += ` AND agent=?`
		args = append(args, agent)
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
