package device

import (
	"context"
	"os"
	"testing"
)

func TestOverviewTotalsReplacementAndFilters(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, a, err := s.Enroll("first")
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := s.Enroll("second")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := sample()
	if _, err = s.Apply(a, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(b, snapshot); err != nil {
		t.Fatal(err)
	}
	all, err := s.Overview(context.Background(), "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if all.Total.Total != 260 || all.Total.Complete || len(all.Devices) != 2 || all.Snapshots != 2 {
		t.Fatalf("%+v", all)
	}
	filtered, err := s.Overview(context.Background(), id, "pi", "2026-01-02", "2026-01-02")
	if err != nil || filtered.Total.Total != 130 {
		t.Fatalf("%+v %v", filtered, err)
	}
	snapshot.Revision++
	snapshot.Totals.Input = 10
	snapshot.Models[0].Input = 10
	snapshot.TotalTokens = 40
	if _, err = s.Apply(a, snapshot); err != nil {
		t.Fatal(err)
	}
	all, err = s.Overview(context.Background(), "", "", "", "")
	if err != nil || all.Total.Total != 170 {
		t.Fatalf("%+v %v", all, err)
	}
	empty, err := s.Overview(context.Background(), "", "codex", "", "")
	if err != nil || empty.Total.Total != 0 || empty.Snapshots != 0 {
		t.Fatalf("%+v %v", empty, err)
	}
}
