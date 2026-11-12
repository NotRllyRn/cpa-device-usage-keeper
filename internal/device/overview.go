package device

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"cpa-usage-keeper/internal/pricing"
)

// Aggregate contains no request counters or performance measurements. Costs are
// known subtotals; Complete must be true before presenting them as full costs.
type Aggregate struct {
	Key      string  `json:"key"`
	Tokens   Tokens  `json:"tokens"`
	Total    int64   `json:"total_tokens"`
	Cost     float64 `json:"known_cost"`
	Complete bool    `json:"complete"`
}
type DailyBreakdown struct {
	Day     string      `json:"day"`
	Models  []Aggregate `json:"models"`
	Devices []Aggregate `json:"devices"`
}
type Overview struct {
	Breakdown   []DailyBreakdown `json:"breakdown"`
	Total       Aggregate        `json:"total"`
	Days        []Aggregate      `json:"days"`
	Models      []Aggregate      `json:"models"`
	Devices     []Aggregate      `json:"devices"`
	Agents      []string         `json:"agents"`
	CollectedAt time.Time        `json:"collected_at"`
	Snapshots   int              `json:"snapshots"`
}

func addTokens(a *Tokens, b Tokens) {
	a.Input += b.Input
	a.Output += b.Output
	a.CacheRead += b.CacheRead
	a.CacheWrite += b.CacheWrite
}
func addAggregate(a *Aggregate, b Aggregate) {
	addTokens(&a.Tokens, b.Tokens)
	a.Total += b.Total
	a.Cost += b.Cost
	a.Complete = a.Complete && b.Complete
}
func bucket(m map[string]*Aggregate, key string) *Aggregate {
	if m[key] == nil {
		m[key] = &Aggregate{Key: key, Complete: true}
	}
	return m[key]
}
func sortedBuckets(m map[string]*Aggregate) []Aggregate {
	out := make([]Aggregate, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Overview reads a consistent SQLite snapshot, independent of browser pagination.
// Limits fail explicitly, never return a silently partial total.
func (s *Store) Overview(ctx context.Context, deviceID, agent, start, end string) (Overview, error) {
	out := Overview{Total: Aggregate{Key: "all", Complete: true}}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	prices := map[string]*pricing.ModelPricingConfig{}
	pr, err := tx.QueryContext(ctx, `SELECT payload FROM device_prices`)
	if err != nil {
		return out, err
	}
	for pr.Next() {
		var raw []byte
		var p pricing.ModelPricingConfig
		if err = pr.Scan(&raw); err == nil {
			err = json.Unmarshal(raw, &p)
		}
		if err != nil {
			pr.Close()
			return out, err
		}
		prices[p.Model] = &p
	}
	err = pr.Err()
	pr.Close()
	if err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT device_id,payload FROM snapshots WHERE (?='' OR device_id=?) AND (?='' OR agent=?) AND (?='' OR day>=?) AND (?='' OR day<=?)`, deviceID, deviceID, agent, agent, start, start, end, end)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	days, models, devices := map[string]*Aggregate{}, map[string]*Aggregate{}, map[string]*Aggregate{}
	agents := map[string]bool{}
	breakdownCells := 0
	dayModels, dayDevices := map[string]map[string]*Aggregate{}, map[string]map[string]*Aggregate{}
	for rows.Next() {
		out.Snapshots++
		if out.Snapshots > 100000 {
			return out, errors.New("range exceeds 100000 snapshots; narrow the filters")
		}
		var id string
		var raw []byte
		var snapshot Snapshot
		if err = rows.Scan(&id, &raw); err == nil {
			err = json.Unmarshal(raw, &snapshot)
		}
		if err != nil {
			return out, err
		}
		agents[snapshot.Agent] = true
		if dayModels[snapshot.Day] == nil {
			dayModels[snapshot.Day] = map[string]*Aggregate{}
			dayDevices[snapshot.Day] = map[string]*Aggregate{}
		}
		if snapshot.CollectedAt.After(out.CollectedAt) {
			out.CollectedAt = snapshot.CollectedAt
		}
		a := Aggregate{Tokens: snapshot.Totals, Total: snapshot.TotalTokens, Complete: true}
		var assigned int64
		for _, m := range snapshot.Models {
			e := estimate(m, resolvePrice(m.Name, prices))
			v := Aggregate{Key: m.Name, Tokens: m.Tokens, Total: m.Input + m.Output + m.CacheRead + m.CacheWrite, Complete: e.Cost != nil}
			assigned += v.Total
			if e.Cost != nil {
				v.Cost = *e.Cost
			}
			a.Cost += v.Cost
			a.Complete = a.Complete && v.Complete
			addAggregate(bucket(models, m.Name), v)
			if dayModels[snapshot.Day][m.Name] == nil {
				breakdownCells++
			}
			if breakdownCells > 100000 {
				return out, errors.New("range exceeds 100000 breakdown cells; narrow the filters")
			}
			addAggregate(bucket(dayModels[snapshot.Day], m.Name), v)
		}
		if assigned != a.Total {
			a.Complete = false
		}
		addAggregate(&out.Total, a)
		addAggregate(bucket(days, snapshot.Day), a)
		addAggregate(bucket(devices, id), a)
		if dayDevices[snapshot.Day][id] == nil {
			breakdownCells++
		}
		if breakdownCells > 100000 {
			return out, errors.New("range exceeds 100000 breakdown cells; narrow the filters")
		}
		addAggregate(bucket(dayDevices[snapshot.Day], id), a)
		if len(models) > 10000 || len(agents) > 1000 {
			return out, errors.New("range has too many identifiers; narrow the filters")
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	out.Days = sortedBuckets(days)
	for _, day := range out.Days {
		out.Breakdown = append(out.Breakdown, DailyBreakdown{Day: day.Key, Models: sortedBuckets(dayModels[day.Key]), Devices: sortedBuckets(dayDevices[day.Key])})
	}
	out.Models = sortedBuckets(models)
	out.Devices = sortedBuckets(devices)
	out.Agents = make([]string, 0, len(agents))
	for a := range agents {
		out.Agents = append(out.Agents, a)
	}
	sort.Strings(out.Agents)
	return out, tx.Commit()
}
