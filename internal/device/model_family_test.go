package device

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"testing"

	"cpa-usage-keeper/internal/pricing"
)

func TestModelFamilyNames(t *testing.T) {
	for family, names := range map[string][]string{
		"openai":    {"gpt-3.5-turbo", "gpt-4o", "gpt-4o-mini-2024-07-18", "gpt-4.1", "gpt-5.4", "gpt-5.3-codex", "gpt-5.3-codex-spark", "gpt-4-0613", "o1", "o1-preview", "o3-mini", "o4-mini-2025-04-16", "codex", "codex-mini-latest", "[pi] gpt-5.4", "[hermes] openai/gpt-5.4", "[codex] openai-codex/gpt-5.3-codex"},
		"anthropic": {"claude-2.1", "claude-instant-1.2", "claude-3-opus-20240229", "claude-3-5-sonnet-20241022", "claude-sonnet-4-5-20250929", "claude-opus-4.6", "claude-haiku-4-5", "[pi] anthropic/claude-opus-4-6", "[hermes] claude-sonnet-4-5"},
		"minimax":   {"MiniMax-M1", "MiniMax-M2", "MiniMax-M2.5", "MiniMax-M2.7-highspeed", "minimax/MiniMax-M2.5", "[pi] MiniMax-M2.7", "[hermes] minimax/MiniMax-M2.5"},
		"unknown":   {"", "pi", "hermes", "openai", "anthropic", "minimax", "gpt-example", "gpt-oss-120b", "gpt-5ish", "gpt-5-clone", "gpt-4o-example", "claude-999-sonnet", "o2", "o3custom", "codex-custom", "my-gpt-5.4", "custom-claude-opus-4-6", "claude-example", "minimax-custom", "foo/MiniMax-M2.5", "[custom] gpt-5.4", "[pi] [pi] gpt-5.4", "anthropic/gpt-5.4", "openai/claude-opus-4-6", "openai/unknown", "openrouter/anthropic/claude-opus-4-6", "path/gpt-5.4", "[pi] custom/openai/gpt-5.4", "claude-sonnet-4-6-copycat/other"},
	} {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				if got := modelFamily(name); got != family {
					t.Fatalf("%q: got %s want %s", name, got, family)
				}
			})
		}
	}
}

func TestModelFamilyAggregationAndBoundary(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := Start(Config{Listen: "127.0.0.1:0", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	idA, a, _ := s.store.Enroll("first")
	idB, b, _ := s.store.Enroll("second")
	put := func(token, agent, day string, models []Model, extraInput, extraTotal int64) {
		t.Helper()
		snapshot := sample()
		snapshot.Agent, snapshot.Day, snapshot.Models = agent, day, models
		snapshot.Totals = Tokens{Input: extraInput}
		snapshot.TotalTokens = extraInput + extraTotal
		for _, m := range models {
			addTokens(&snapshot.Totals, m.Tokens)
			snapshot.TotalTokens += m.Input + m.Output + m.CacheRead + m.CacheWrite
		}
		if _, err := s.store.Apply(token, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	put(a, "pi", "2024-01-02", []Model{
		{Name: "[pi] gpt-5.4", Tokens: Tokens{Input: 100, Output: 20, CacheRead: 5, CacheWrite: 2}},
		{Name: "[pi] anthropic/claude-sonnet-4-5", Tokens: Tokens{Input: 40, Output: 10}},
		{Name: "[pi] MiniMax-M2.7", Tokens: Tokens{Input: 30}},
		{Name: "mystery", Tokens: Tokens{Input: 11}},
	}, 7, 6)
	put(b, "hermes", "2024-01-03", []Model{
		{Name: "gpt-5.4", Tokens: Tokens{Output: 3}},
		{Name: "claude-opus-4-6", Tokens: Tokens{Input: 5}},
		{Name: "MiniMax-M2.5", Tokens: Tokens{Output: 9}},
	}, 0, 0)
	put(a, "custom-worker", "2024-01-04", []Model{
		{Name: "openai/o3-mini", Tokens: Tokens{Input: 4}},
		{Name: "anthropic/claude-3-5-sonnet-20241022", Tokens: Tokens{Input: 2}},
		{Name: "minimax/MiniMax-M2.5", Tokens: Tokens{Output: 6}},
	}, 0, 0)
	// An agent called codex is not evidence of an OpenAI model.
	put(b, "codex", "2023-01-01", []Model{{Name: "claude-haiku-4-5", Tokens: Tokens{Input: 8}}}, 0, 0)
	put(a, "old", "2020-01-01", []Model{}, 2, 3)
	p := pricing.ModelPricingConfig{Model: "gpt-5.4", ConditionalMultipliers: []pricing.RuleConfig{}, Branches: []pricing.PriceBranch{}, PricingStyle: "openai", ModelMultiplier: 1, BasePrices: pricing.BasePrices{Input: 1, Output: 1, CacheRead: 1, CacheWrite: 1}}
	if err := s.store.SavePrice(p); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var reconstructed Aggregate
	reconstructed.Complete = true
	for _, tc := range []struct {
		family                     string
		total                      int64
		snapshots, priced, missing int
		unattributed               int64
		cost                       float64
		boundary                   string
	}{
		{"", 273, 5, 2, 9, 18, .000130, "2020-01-01"},
		{"openai", 134, 3, 2, 1, 0, .000130, "2024-01-02"},
		{"anthropic", 65, 4, 0, 4, 0, 0, "2023-01-01"},
		{"minimax", 45, 3, 0, 3, 0, 0, "2024-01-02"},
		{"unknown", 29, 2, 0, 1, 18, 0, "2020-01-01"},
	} {
		t.Run("family="+tc.family, func(t *testing.T) {
			out, err := s.store.Analysis(ctx, "", "", "", "", tc.family)
			if err != nil {
				t.Fatal(err)
			}
			if out.Total.Total != tc.total || out.Snapshots != tc.snapshots || out.Total.Coverage.Priced != tc.priced || out.Total.Coverage.Missing != tc.missing || out.Total.Coverage.Unattributed != tc.unattributed || out.Total.Complete || math.Abs(out.Total.Cost-tc.cost) > 1e-12 {
				t.Fatalf("%+v", out)
			}
			// Every view partitions the same exclusive model categories/remainder.
			check := func(values []Aggregate) {
				t.Helper()
				sum := Aggregate{Key: "all", Complete: true}
				for _, v := range values {
					addAggregate(&sum, v)
				}
				if !reflect.DeepEqual(sum, out.Total) {
					t.Fatalf("partition %+v != %+v", sum, out.Total)
				}
			}
			check(out.Models)
			check(out.Devices)
			check(out.Days)
			cellValues := []Aggregate{}
			for _, cell := range out.Cells {
				cellValues = append(cellValues, cell.Aggregate)
			}
			check(cellValues)
			for _, day := range out.Breakdown {
				var models, devices int64
				for _, v := range day.Models {
					models += v.Total
				}
				for _, v := range day.Devices {
					devices += v.Total
				}
				if models != devices {
					t.Fatalf("day mismatch %+v", day)
				}
			}
			overview, err := s.store.Overview(ctx, "", "", "", "", tc.family)
			if err != nil || !reflect.DeepEqual(overview.Total, out.Total) || !reflect.DeepEqual(overview.Models, out.Models) {
				t.Fatalf("overview mismatch %+v %v", overview, err)
			}
			day, err := s.store.EarliestUsageDay(ctx, "", "", tc.family)
			if err != nil || day == nil || *day != tc.boundary {
				t.Fatalf("boundary %v %v", day, err)
			}
			if tc.family != "" {
				addAggregate(&reconstructed, out.Total)
			}
		})
	}
	all, _ := s.store.Analysis(ctx, "", "", "", "", "")
	reconstructed.Key = "all"
	if !reflect.DeepEqual(reconstructed, all.Total) {
		t.Fatalf("families %+v != all %+v", reconstructed, all.Total)
	}
	for _, tc := range []struct {
		device, agent, start, end, family string
		total                             int64
	}{
		{idA, "pi", "", "", "openai", 127}, {idB, "", "", "", "openai", 3},
		{"", "codex", "", "", "openai", 0}, {"", "hermes", "", "", "minimax", 9},
		{"", "", "2024-01-03", "2024-01-04", "openai", 7},
		{"", "", "", "2024-01-01", "unknown", 5},
	} {
		out, err := s.store.Analysis(ctx, tc.device, tc.agent, tc.start, tc.end, tc.family)
		if err != nil || out.Total.Total != tc.total {
			t.Fatalf("%+v: %+v %v", tc, out, err)
		}
	}
	for _, tc := range []struct{ device, agent, family, want string }{
		{idB, "", "openai", "2024-01-03"}, {idA, "pi", "minimax", "2024-01-02"},
		{idB, "codex", "openai", ""}, {idB, "hermes", "unknown", ""},
	} {
		got, err := s.store.EarliestUsageDay(ctx, tc.device, tc.agent, tc.family)
		if err != nil || tc.want == "" && got != nil || tc.want != "" && (got == nil || *got != tc.want) {
			t.Fatalf("%+v: %v %v", tc, got, err)
		}
	}
	// A sole named model cannot absorb aggregate extras. Unknown owns the delta.
	put(a, "sole", "2021-01-01", []Model{{Name: "gpt-5.4"}}, 2, 3)
	sole, err := s.store.Analysis(ctx, "", "sole", "", "", "openai")
	if err != nil || sole.Total.Total != 0 || !sole.Total.Complete || sole.Total.Coverage.Unattributed != 0 {
		t.Fatalf("%+v %v", sole, err)
	}
	day, err := s.store.EarliestUsageDay(ctx, "", "", "openai")
	if err != nil || day == nil || *day != "2021-01-01" {
		t.Fatalf("zero boundary %v %v", day, err)
	}
	// HTTP validation/auth is shared by both views and the All time boundary.
	for _, path := range []string{"/v1/overview", "/v1/analysis", "/v1/usage-boundary"} {
		for _, tc := range []struct {
			token, family string
			status        int
		}{
			{"", "openai", 401}, {a, "openai", 401}, {s.admin, "openai", 200}, {s.admin, "made-up", 400},
		} {
			req, _ := http.NewRequest("GET", "http://"+s.Address()+path+"?family="+tc.family, nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.status {
				t.Fatalf("%s %s: %d", path, tc.family, response.StatusCode)
			}
			if tc.status == 200 && path != "/v1/usage-boundary" {
				var out Overview
				if err := json.NewDecoder(response.Body).Decode(&out); err != nil || out.Total.Total != 134 {
					t.Fatalf("HTTP %+v %v", out, err)
				}
			}
			response.Body.Close()
		}
	}
	// Exact Pi overrides, fallback rates and conditional/missing prices change
	// cost coverage only, never model identity, family or token selection.
	p.Model, p.ModelMultiplier = "[pi] gpt-5.4", 0
	if err := s.store.SavePrice(p); err != nil {
		t.Fatal(err)
	}
	override, err := s.store.Analysis(ctx, "", "", "2024-01-01", "", "openai")
	if err != nil || override.Total.Total != 134 || math.Abs(override.Total.Cost-.000003) > 1e-12 || override.Total.Coverage.Priced != 2 || override.Models[0].Key != "[pi] gpt-5.4" {
		t.Fatalf("override %+v %v", override, err)
	}
	p.Model, p.ModelMultiplier = "gpt-5.4", 1
	p.Branches = []pricing.PriceBranch{{ID: "conditional", Name: "Conditional", Days: pricing.DaysWeekday, Context: pricing.ContextCondition{Type: pricing.ContextAll}, Period: pricing.PeriodCondition{Type: pricing.PeriodAll}, Prices: p.BasePrices}}
	if err := s.store.SavePrice(p); err != nil {
		t.Fatal(err)
	}
	conditional, err := s.store.Analysis(ctx, "", "", "2024-01-01", "", "openai")
	if err != nil || conditional.Total.Total != 134 || conditional.Total.Coverage.Conditional != 1 || conditional.Total.Coverage.Priced != 1 || conditional.Total.Coverage.Missing != 1 || conditional.Total.Cost != 0 {
		t.Fatalf("conditional %+v %v", conditional, err)
	}
	for _, model := range []string{"[pi] gpt-5.4", "gpt-5.4"} {
		if err := s.store.DeletePrice(model); err != nil {
			t.Fatal(err)
		}
	}
	missing, err := s.store.Analysis(ctx, "", "", "2024-01-01", "", "openai")
	if err != nil || missing.Total.Total != 134 || missing.Total.Coverage.Missing != 3 || missing.Total.Complete || missing.Total.Cost != 0 {
		t.Fatalf("missing %+v %v", missing, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.store.EarliestUsageDay(cancelled, "", "", "openai"); err == nil {
		t.Fatal("ignored boundary cancellation")
	}
	if _, err := s.store.Analysis(cancelled, "", "", "", "", "openai"); err == nil {
		t.Fatal("ignored aggregation cancellation")
	}
}

func TestModelFamilyZeroDays(t *testing.T) {
	for _, tc := range []struct {
		models []Model
		family string
		want   bool
	}{
		{nil, "", true}, {nil, "unknown", true}, {nil, "openai", false},
		{[]Model{{Name: "gpt-5.4"}}, "openai", true},
		{[]Model{{Name: "gpt-5.4"}}, "unknown", false},
		{[]Model{{Name: "custom"}}, "unknown", true},
	} {
		_, remainder, matches := selectModelFamily(Snapshot{Models: tc.models}, tc.family)
		if matches != tc.want || remainder.Total != 0 {
			t.Fatalf("%+v: matches=%v remainder=%+v", tc, matches, remainder)
		}
	}
}

func TestModelFamilyAnalysisBounds(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, token, err := s.Enroll("bounds")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := sample()
	snapshot.Models = nil
	for i := 0; i < 100; i++ {
		snapshot.Models = append(snapshot.Models, Model{Name: "gpt-5." + strconv.Itoa(i)})
	}
	if _, err := s.Apply(token, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Day = "2026-01-03"
	snapshot.Models = []Model{{Name: "gpt-5.100"}}
	if _, err := s.Apply(token, snapshot); err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"", "openai"} {
		if _, err := s.Analysis(context.Background(), "", "", "", "", family); err == nil {
			t.Fatalf("%q silently accepted oversized analysis", family)
		}
	}
	if out, err := s.Analysis(context.Background(), "", "", "", "", "minimax"); err != nil || out.Snapshots != 0 {
		t.Fatalf("unmatched dimensions: %+v %v", out, err)
	}
}
