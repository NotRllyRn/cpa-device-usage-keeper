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
type CostCoverage struct {
	Priced       int   `json:"priced"`
	Missing      int   `json:"missing"`
	Conditional  int   `json:"conditional"`
	Unattributed int64 `json:"unattributed_tokens"`
}

func (a *CostCoverage) add(b CostCoverage) {
	a.Priced += b.Priced
	a.Missing += b.Missing
	a.Conditional += b.Conditional
	a.Unattributed += b.Unattributed
}

type Aggregate struct {
	Coverage CostCoverage `json:"coverage"`
	Key      string       `json:"key"`
	Tokens   Tokens       `json:"tokens"`
	Total    int64        `json:"total_tokens"`
	Cost     float64      `json:"known_cost"`
	Complete bool         `json:"complete"`
}
type DailyBreakdown struct {
	Day     string      `json:"day"`
	Models  []Aggregate `json:"models"`
	Devices []Aggregate `json:"devices"`
}
type AnalysisCell struct {
	Device string `json:"device"`
	Model  string `json:"model"`
	Aggregate
}

type Overview struct {
	Cells       []AnalysisCell   `json:"cells,omitempty"`
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
	a.Coverage.add(b.Coverage)
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
func (s *Store) Overview(ctx context.Context, deviceID, agent, start, end, family string) (Overview, error) {
	return s.aggregate(ctx, deviceID, agent, start, end, family, false)
}

// Analysis shares the consistent snapshot and Keeper pricing path with Overview.
// The extra device/model matrix is collected only for this authenticated endpoint.
func (s *Store) Analysis(ctx context.Context, deviceID, agent, start, end, family string) (Overview, error) {
	return s.aggregate(ctx, deviceID, agent, start, end, family, true)
}

func (s *Store) aggregate(ctx context.Context, deviceID, agent, start, end, family string, analysis bool) (Overview, error) {
	out := Overview{Total: Aggregate{Key: "all", Complete: true}}
	if !validModelFamily(family) {
		return out, errors.New("invalid model family")
	}
	cells := map[string]map[string]*Aggregate{}
	var payloadBytes int64
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
	rows, err := tx.QueryContext(ctx, `SELECT device_id,payload FROM snapshots WHERE (?='' OR device_id=?) AND (?='' OR agent=?) AND (?='' OR day>=?) AND (?='' OR day<=?) LIMIT 100001`, deviceID, deviceID, agent, agent, start, start, end, end)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	days, models, devices := map[string]*Aggregate{}, map[string]*Aggregate{}, map[string]*Aggregate{}
	agents := map[string]bool{}
	breakdownCells, scanned := 0, 0
	dayModels, dayDevices := map[string]map[string]*Aggregate{}, map[string]map[string]*Aggregate{}
	for rows.Next() {
		scanned++
		if scanned > 100000 {
			return out, errors.New("range exceeds 100000 snapshots; narrow the filters")
		}
		var id string
		var raw []byte
		var snapshot Snapshot
		if err = rows.Scan(&id, &raw); err == nil {
			payloadBytes += int64(len(raw))
			if payloadBytes > 256<<20 {
				return out, errors.New("range exceeds 256 MiB; narrow the filters")
			}
			err = json.Unmarshal(raw, &snapshot)
		}
		if err != nil {
			return out, err
		}
		agents[snapshot.Agent] = true
		if len(agents) > 1000 {
			return out, errors.New("range has too many identifiers; narrow the filters")
		}
		selected, remainder, matches := selectModelFamily(snapshot, family)
		if !matches {
			continue
		}
		out.Snapshots++
		if dayModels[snapshot.Day] == nil {
			dayModels[snapshot.Day] = map[string]*Aggregate{}
			dayDevices[snapshot.Day] = map[string]*Aggregate{}
		}
		if snapshot.CollectedAt.After(out.CollectedAt) {
			out.CollectedAt = snapshot.CollectedAt
		}
		a := Aggregate{Complete: true}
		if analysis && cells[id] == nil {
			cells[id] = map[string]*Aggregate{}
		}
		for _, m := range selected {
			e := estimate(m, resolvePrice(m.Name, prices))
			v := Aggregate{Key: m.Name, Tokens: m.Tokens, Total: m.Input + m.Output + m.CacheRead + m.CacheWrite, Complete: e.Cost != nil}
			if e.Cost != nil {
				v.Cost = *e.Cost
				v.Coverage.Priced = 1
			} else if e.Status == "missing_price" {
				v.Coverage.Missing = 1
			} else {
				v.Coverage.Conditional = 1
			}
			addAggregate(&a, v)
			if analysis {
				addAggregate(bucket(cells[id], m.Name), v)
			}
			addAggregate(bucket(models, m.Name), v)
			if dayModels[snapshot.Day][m.Name] == nil {
				breakdownCells++
			}
			if breakdownCells > 100000 {
				return out, errors.New("range exceeds 100000 breakdown cells; narrow the filters")
			}
			addAggregate(bucket(dayModels[snapshot.Day], m.Name), v)
		}
		if remainder.Total > 0 {
			addAggregate(&a, remainder)
			// Empty model identifier cannot occur in validated uploads. Use the
			// same honest remainder bucket in Overview and Analysis rankings.
			if analysis {
				addAggregate(bucket(cells[id], ""), remainder)
			}
			addAggregate(bucket(models, ""), remainder)
			if dayModels[snapshot.Day][""] == nil {
				breakdownCells++
			}
			addAggregate(bucket(dayModels[snapshot.Day], ""), remainder)
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
		if analysis && (len(models) > 100 || len(devices) > 100 || len(days)*(len(models)+len(devices)) > 100000) {
			return out, errors.New("analysis exceeds 100 models/devices or 100000 chart points; narrow the filters")
		}
		if len(models) > 10000 {
			return out, errors.New("range has too many identifiers; narrow the filters")
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	for _, device := range sortedBuckets(devices) {
		for _, value := range sortedBuckets(cells[device.Key]) {
			out.Cells = append(out.Cells, AnalysisCell{Device: device.Key, Model: value.Key, Aggregate: value})
		}
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
