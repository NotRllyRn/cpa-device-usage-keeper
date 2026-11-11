package device

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sample() Snapshot {
	return Snapshot{Agent: "pi", Day: "2026-01-02", Revision: 1, CollectedAt: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Totals: Tokens{Input: 100, Output: 20, CacheRead: 10}, TotalTokens: 130, Models: []Model{{Name: "model-test", Tokens: Tokens{Input: 100, Output: 20, CacheRead: 10}}}}
}
func TestReplacementAndRevocation(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, token, err := s.Enroll("laptop")
	if err != nil {
		t.Fatal(err)
	}
	a := sample()
	if err = a.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	if replay, err := s.Apply(token, a); err != nil || replay {
		t.Fatalf("first %v %v", replay, err)
	}
	if replay, err := s.Apply(token, a); err != nil || !replay {
		t.Fatalf("replay %v %v", replay, err)
	}
	a.Models[0].Input = 1
	if _, err = s.Apply(token, a); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	a.Revision = 2
	if _, err = s.Apply(token, a); err != nil {
		t.Fatal(err)
	}
	a.Revision = 1
	if _, err = s.Apply(token, a); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.History(0)
	if err != nil || len(rows) != 1 || rows[0].Models[0].Input != 1 {
		t.Fatalf("history %+v %v", rows, err)
	}
	if err = s.Revoke(id); err != nil {
		t.Fatal(err)
	}
	a.Revision = 3
	if _, err = s.Apply(token, a); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
}
func TestEscapedPathAndBackfillCursor(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state?mode=memory#test")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := s.Enroll("test")
	if err != nil {
		t.Fatal(err)
	}
	a := sample()
	if _, err = s.Apply(token, a); err != nil {
		t.Fatal(err)
	}
	first, err := s.History(0)
	if err != nil || len(first) != 1 {
		t.Fatalf("%v %v", first, err)
	}
	a.Day = "2026-01-01"
	if _, err = s.Apply(token, a); err != nil {
		t.Fatal(err)
	}
	next, err := s.History(int(first[0].Cursor))
	if err != nil || len(next) != 1 || next[0].Day != a.Day {
		t.Fatalf("%v %v", next, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.History(0)
	if err != nil || len(rows) != 2 {
		t.Fatalf("escaped path was not durable: %v %v", rows, err)
	}
}
func TestValidation(t *testing.T) {
	for _, change := range []func(*Snapshot){func(s *Snapshot) { s.Models[0].Input = -1 }, func(s *Snapshot) { s.Models[0].Name = "invalid model\n" }, func(s *Snapshot) { s.Day = "2026-02-30" }, func(s *Snapshot) { s.Models = append(s.Models, s.Models[0]) }, func(s *Snapshot) { s.Revision = 0 }, func(s *Snapshot) { s.CollectedAt = time.Now().Add(time.Hour) }} {
		a := sample()
		change(&a)
		if a.Validate(time.Now()) == nil {
			t.Fatal("accepted invalid snapshot")
		}
	}
}
