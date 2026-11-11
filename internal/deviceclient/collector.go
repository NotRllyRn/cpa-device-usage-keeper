package deviceclient

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"cpa-usage-keeper/internal/device"
)

// ParseDaily consumes ccusage reports, not original agent logs. Only whitelisted
// numeric fields and identifiers leave the device; collector costs are ignored.
func ParseDaily(raw []byte, now time.Time) ([]device.Snapshot, error) {
	type counts struct {
		Input  *int64 `json:"inputTokens"`
		Output *int64 `json:"outputTokens"`
		Read   *int64 `json:"cacheReadTokens"`
		Write  *int64 `json:"cacheCreationTokens"`
	}
	type model struct {
		Name string `json:"modelName"`
		counts
	}
	type agent struct {
		Name string `json:"agent"`
		counts
		Total  *int64  `json:"totalTokens"`
		Models []model `json:"modelBreakdowns"`
	}
	var report struct {
		Daily *[]struct {
			Day    string  `json:"period"`
			Agent  string  `json:"agent"`
			Agents []agent `json:"agents"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(raw, &report); err != nil || report.Daily == nil {
		return nil, errors.New("unsupported collector schema: daily array required")
	}
	tokens := func(c counts) (device.Tokens, error) {
		if c.Input == nil || c.Output == nil || c.Read == nil || c.Write == nil {
			return device.Tokens{}, errors.New("unsupported collector schema: token category missing")
		}
		return device.Tokens{Input: *c.Input, Output: *c.Output, CacheRead: *c.Read, CacheWrite: *c.Write}, nil
	}
	out := []device.Snapshot{}
	seen := map[string]bool{}
	for _, day := range *report.Daily {
		if day.Agent != "all" || day.Agents == nil {
			return nil, errors.New("unsupported collector schema: by-agent report required")
		}
		for _, a := range day.Agents {
			key := a.Name + "/" + day.Day
			if seen[key] {
				return nil, errors.New("duplicate collector day/agent")
			}
			seen[key] = true
			snapshot := device.Snapshot{Agent: a.Name, Day: day.Day, Revision: 1, CollectedAt: now, Models: []device.Model{}}
			total, err := tokens(a.counts)
			if err != nil {
				return nil, err
			}
			if a.Total == nil {
				return nil, errors.New("missing totalTokens")
			}
			snapshot.Totals = total
			snapshot.TotalTokens = *a.Total
			for _, m := range a.Models {
				t, err := tokens(m.counts)
				if err != nil {
					return nil, err
				}
				snapshot.Models = append(snapshot.Models, device.Model{Name: m.Name, Tokens: t})
			}
			// Totals may include unattributed categories and extra tokens. Preserve
			// them independently; never invent a model or price for the difference.
			sort.Slice(snapshot.Models, func(i, j int) bool { return snapshot.Models[i].Name < snapshot.Models[j].Name })
			if err := snapshot.Validate(now); err != nil {
				return nil, errors.New("collector report failed validation")
			}
			out = append(out, snapshot)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day == out[j].Day {
			return out[i].Agent < out[j].Agent
		}
		return out[i].Day < out[j].Day
	})
	return out, nil
}
