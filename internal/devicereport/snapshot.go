// Package devicereport defines the portable device upload contract.
package devicereport

import (
	"errors"
	"regexp"
	"time"
)

const MaxTokens int64 = 1_000_000_000_000

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:+-]{0,159}$`)
var modelIdentifier = regexp.MustCompile(`^[a-zA-Z0-9\[][a-zA-Z0-9/._:+\[\] -]{0,159}$`)

// Tokens use disjoint categories: Input excludes cache reads/writes, Output
// follows the collector output bucket; reasoning is not reconstructed. No cost or request count supplied by a collector is used.
type Tokens struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}
type Model struct {
	Name string `json:"model"`
	Tokens
}
type Snapshot struct {
	Agent       string    `json:"agent"`
	Day         string    `json:"day"`
	Revision    int64     `json:"revision"`
	CollectedAt time.Time `json:"collected_at"`
	Models      []Model   `json:"models"`
	Totals      Tokens    `json:"totals"`
	TotalTokens int64     `json:"total_tokens"`
}

func (s Snapshot) Validate(now time.Time) error {
	day, err := time.Parse("2006-01-02", s.Day)
	if err != nil || day.Format("2006-01-02") != s.Day || day.After(now.UTC()) || day.Year() < 2000 {
		return errors.New("invalid day")
	}
	if !identifier.MatchString(s.Agent) || s.Revision < 1 || s.Revision > MaxTokens {
		return errors.New("invalid agent or revision")
	}
	if s.CollectedAt.IsZero() || s.CollectedAt.After(now.Add(5*time.Minute)) || s.CollectedAt.Before(day) {
		return errors.New("invalid collection time")
	}
	if s.Models == nil || len(s.Models) > 100 {
		return errors.New("models must be an array of at most 100 entries")
	}
	for _, n := range []int64{s.Totals.Input, s.Totals.Output, s.Totals.CacheRead, s.Totals.CacheWrite, s.TotalTokens} {
		if n < 0 || n > MaxTokens {
			return errors.New("invalid total tokens")
		}
	}
	if s.TotalTokens < s.Totals.Input+s.Totals.Output+s.Totals.CacheRead+s.Totals.CacheWrite {
		return errors.New("total tokens below categories")
	}
	var sum Tokens
	seen := map[string]bool{}
	for _, m := range s.Models {
		if !modelIdentifier.MatchString(m.Name) || seen[m.Name] {
			return errors.New("invalid or duplicate model")
		}
		seen[m.Name] = true
		for _, n := range []int64{m.Input, m.Output, m.CacheRead, m.CacheWrite} {
			if n < 0 || n > MaxTokens {
				return errors.New("invalid token count")
			}
		}
		sum.Input += m.Input
		sum.Output += m.Output
		sum.CacheRead += m.CacheRead
		sum.CacheWrite += m.CacheWrite
	}
	if sum.Input > s.Totals.Input || sum.Output > s.Totals.Output || sum.CacheRead > s.Totals.CacheRead || sum.CacheWrite > s.Totals.CacheWrite {
		return errors.New("model categories exceed totals")
	}
	return nil
}
