package device

import (
	"context"
	"cpa-usage-keeper/internal/pricing"
	"math"
	"os"
	"testing"
)

func TestAnalysisMatrixPricingAndFilters(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, a, _ := s.Enroll("first")
	_, b, _ := s.Enroll("second")
	snapshot := sample()
	snapshot.Models[0].Name = "[pi] model-test"
	snapshot.TotalTokens += 7 // Retain tokens lacking a category/model.
	if _, err = s.Apply(a, snapshot); err != nil {
		t.Fatal(err)
	}
	other := sample()
	other.Agent = "codex"
	other.Day = "2026-01-03"
	if _, err = s.Apply(b, other); err != nil {
		t.Fatal(err)
	}
	p := pricing.ModelPricingConfig{Model: "model-test", ConditionalMultipliers: []pricing.RuleConfig{}, Branches: []pricing.PriceBranch{}, PricingStyle: "openai", ModelMultiplier: 1, BasePrices: pricing.BasePrices{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}}
	if err = s.SavePrice(p); err != nil {
		t.Fatal(err)
	}
	read := func(device, agent, start, end string) Overview {
		t.Helper()
		out, err := s.Analysis(context.Background(), device, agent, start, end)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	all := read("", "", "", "")
	if all.Total.Coverage.Priced != 2 || all.Total.Coverage.Unattributed != 7 || all.Total.Total != 267 || all.Total.Complete || math.Abs(all.Total.Cost-.00034) > 1e-12 || len(all.Cells) != 3 {
		t.Fatalf("%+v", all)
	}
	filtered := read(id, "pi", "2026-01-02", "2026-01-02")
	if filtered.Total.Total != 137 || len(filtered.Cells) != 2 || filtered.Cells[0].Model != "" || filtered.Cells[0].Total != 7 || filtered.Cells[0].Complete {
		t.Fatalf("%+v", filtered)
	}
	if got := read(id, "codex", "", ""); got.Total.Total != 0 || len(got.Cells) != 0 {
		t.Fatal(got)
	}
	if got := read("", "", "2026-01-04", "2026-01-05"); got.Snapshots != 0 {
		t.Fatal(got)
	}
	// Exact Pi override wins; explicit free remains complete, unlike missing prices.
	p.Model = "[pi] model-test"
	p.ModelMultiplier = 0
	if err = s.SavePrice(p); err != nil {
		t.Fatal(err)
	}
	got := read(id, "", "", "")
	if got.Cells[1].Coverage.Priced != 1 || got.Cells[1].Cost != 0 || !got.Cells[1].Complete {
		t.Fatal(got)
	}
	p.Branches = []pricing.PriceBranch{{ID: "conditional", Name: "Conditional", Days: pricing.DaysWeekday, Context: pricing.ContextCondition{Type: pricing.ContextAll}, Period: pricing.PeriodCondition{Type: pricing.PeriodAll}, Prices: p.BasePrices}}
	// The estimator cannot evaluate conditional rules from daily input.
	// Persist a valid conditional config through the same storage representation.
	if err = s.SavePrice(p); err != nil {
		t.Fatal(err)
	}
	got = read(id, "", "", "")
	if got.Cells[1].Complete || got.Cells[1].Coverage.Conditional != 1 || got.Cells[1].Coverage.Priced != 0 {
		t.Fatal(got)
	}
	if err = s.DeletePrice(p.Model); err != nil {
		t.Fatal(err)
	}
	if err = s.DeletePrice("model-test"); err != nil {
		t.Fatal(err)
	}
	got = read(id, "", "", "")
	if got.Cells[1].Coverage.Missing != 1 || got.Cells[1].Coverage.Priced != 0 || got.Cells[1].Complete || got.Cells[1].Cost != 0 {
		t.Fatal(got)
	}
	overview, err := s.Overview(context.Background(), "", "", "", "")
	if err != nil || len(overview.Cells) != 0 || len(overview.Models) != 2 {
		t.Fatalf("overview changed: %+v %v", overview, err)
	}
}

func TestAnalysisRejectsOversizedDimensions(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, token, _ := s.Enroll("first")
	snapshot := sample()
	snapshot.Models = nil
	for i := 0; i < 100; i++ {
		snapshot.Models = append(snapshot.Models, Model{Name: fmtModel(i)})
	}
	if _, err = s.Apply(token, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Day = "2026-01-03"
	snapshot.Models = []Model{{Name: "extra"}}
	if _, err = s.Apply(token, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Analysis(context.Background(), "", "", "", ""); err == nil {
		t.Fatal("oversized analysis silently accepted")
	}
}

func fmtModel(i int) string { return "model-" + string(rune('A'+i/26)) + string(rune('A'+i%26)) }
