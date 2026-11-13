package deviceclient

import (
	"context"
	device "cpa-usage-keeper/internal/devicereport"
	"errors"
	"math"
	"path/filepath"
	"sort"
	"time"
)

// Collect uses one invocation for multi-root adapters. Adapters whose upstream
// override accepts only one path get additional isolated invocations. We merge
// only sanitized ccusage aggregates, never parse original usage files.
func Collect(ctx context.Context, c Config) ([]device.Snapshot, error) {
	if err := ValidateSources(c.Sources); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var all []device.Snapshot
	for _, batch := range sourceBatches(c.Sources) {
		current := c
		current.Sources = batch
		snapshots, err := collectBatch(ctx, current)
		if err != nil {
			return nil, err
		}
		all = append(all, snapshots...)
	}
	return mergeSnapshots(all)
}

func sourceBatches(sources []Source) [][]Source {
	batches := [][]Source{{}}
	singles := map[string]bool{}
	for _, s := range sources {
		// The exporter can be inside any selected Copilot home's otel tree.
		// Avoid counting it again in a different single-home invocation.
		covered := false
		if s.Agent == "copilot-otel" {
			for _, root := range sources {
				if root.Agent == "copilot" {
					rel, err := filepath.Rel(filepath.Join(root.Path, "otel"), s.Path)
					if err == nil && inside(rel) {
						covered = true
					}
				}
			}
		}
		if covered {
			continue
		}
		if sourceSpecs[s.Agent].single && singles[s.Agent] {
			batches = append(batches, []Source{s})
		} else {
			batches[0] = append(batches[0], s)
			singles[s.Agent] = true
		}
	}
	return batches
}

func mergeSnapshots(all []device.Snapshot) ([]device.Snapshot, error) {
	byKey := map[string]device.Snapshot{}
	add := func(a, b int64) (int64, error) {
		if a < 0 || b < 0 || a > math.MaxInt64-b {
			return 0, errors.New("collector aggregate overflow")
		}
		return a + b, nil
	}
	addTokens := func(a, b device.Tokens) (device.Tokens, error) {
		var result device.Tokens
		var err error
		if result.Input, err = add(a.Input, b.Input); err != nil {
			return result, err
		}
		if result.Output, err = add(a.Output, b.Output); err != nil {
			return result, err
		}
		if result.CacheRead, err = add(a.CacheRead, b.CacheRead); err != nil {
			return result, err
		}
		if result.CacheWrite, err = add(a.CacheWrite, b.CacheWrite); err != nil {
			return result, err
		}
		return result, nil
	}
	for _, s := range all {
		key := s.Agent + "/" + s.Day
		old, ok := byKey[key]
		if !ok {
			byKey[key] = s
			continue
		}
		var err error
		if old.Totals, err = addTokens(old.Totals, s.Totals); err != nil {
			return nil, err
		}
		if old.TotalTokens, err = add(old.TotalTokens, s.TotalTokens); err != nil {
			return nil, err
		}
		models := map[string]device.Tokens{}
		for _, m := range old.Models {
			models[m.Name] = m.Tokens
		}
		for _, m := range s.Models {
			tokens, err := addTokens(models[m.Name], m.Tokens)
			if err != nil {
				return nil, err
			}
			models[m.Name] = tokens
		}
		old.Models = nil
		for name, tokens := range models {
			old.Models = append(old.Models, device.Model{Name: name, Tokens: tokens})
		}
		sort.Slice(old.Models, func(i, j int) bool { return old.Models[i].Name < old.Models[j].Name })
		if err = old.Validate(time.Now().UTC()); err != nil {
			return nil, err
		}
		byKey[key] = old
	}
	result := make([]device.Snapshot, 0, len(byKey))
	for _, s := range byKey {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Day != result[j].Day {
			return result[i].Day < result[j].Day
		}
		return result[i].Agent < result[j].Agent
	})
	return result, nil
}
