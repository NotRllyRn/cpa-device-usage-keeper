package deviceclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	device "cpa-usage-keeper/internal/devicereport"
)

func sourceDir(t *testing.T, home, relative string) string {
	t.Helper()
	p := filepath.Join(home, relative)
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	p, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestActualPiEnvironmentDiscovery(t *testing.T) {
	home := os.Getenv("DEVICE_USAGE_TEST_DISCOVERY_HOME")
	if home == "" {
		t.Skip("opt-in metadata-only discovery on this machine")
	}
	root := os.Getenv("PI_CODING_AGENT_DIR")
	if root == "" {
		t.Fatal("PI_CODING_AGENT_DIR required")
	}
	expected, err := canonicalSource(Source{"pi", filepath.Join(root, "sessions")}, home)
	if err != nil {
		t.Fatal(err)
	}
	sources, _ := DiscoverSources(home, os.Getenv)
	for _, s := range sources {
		if s == expected {
			return
		}
	}
	t.Fatal("custom Pi agent sessions not discovered")
}

func TestDiscoveryKnownAndCustomRoots(t *testing.T) {
	home := t.TempDir()
	pi := sourceDir(t, home, "custom pi/agent/sessions")
	standard := sourceDir(t, home, ".pi/agent/sessions")
	claude := sourceDir(t, home, "custom claude/projects")
	opencode := sourceDir(t, home, "data/opencode")
	env := map[string]string{"PI_CODING_AGENT_DIR": filepath.Dir(pi), "PI_AGENT_DIR": pi, "CLAUDE_CONFIG_DIR": claude, "XDG_DATA_HOME": filepath.Dir(opencode)}
	found, warnings := DiscoverSources(home, func(k string) string { return env[k] })
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	want := []Source{{"claude", filepath.Dir(claude)}, {"opencode", opencode}, {"pi", standard}, {"pi", pi}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("got %+v want %+v", found, want)
	}
}
func TestSelectionDefaultsTogglesManualAndEOF(t *testing.T) {
	home := t.TempDir()
	a := Source{"pi", sourceDir(t, home, "a")}
	b := Source{"pi", sourceDir(t, home, "b")}
	for _, tt := range []struct {
		input string
		want  []Source
	}{{"\n\n", []Source{a, b}}, {"2\n\n\n", []Source{a}}, {"1 2\n\n\n", []Source{}}} {
		var out bytes.Buffer
		got, err := SelectSources(bufio.NewReader(strings.NewReader(tt.input)), &out, home, []Source{a, b})
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%+v %v", got, err)
		}
		if !strings.Contains(out.String(), "Selected roots:") || !strings.Contains(out.String(), "[x]") {
			t.Fatal(out.String())
		}
	}
	var out bytes.Buffer
	got, err := SelectSources(bufio.NewReader(strings.NewReader("add pi "+a.Path+"\n\n\n")), &out, home, nil)
	if err != nil || !reflect.DeepEqual(got, []Source{a}) {
		t.Fatalf("manual: %+v %v", got, err)
	}
	if _, err = SelectSources(bufio.NewReader(strings.NewReader("")), &out, home, []Source{a}); err == nil {
		t.Fatal("EOF must not consent")
	}
}
func TestSourcesPersistAndPendingBlocksChanges(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	a := Source{"pi", sourceDir(t, dir, "a")}
	b := Source{"pi", sourceDir(t, dir, "b")}
	c := Config{URL: "https://example.invalid", Token: strings.Repeat("a", 64), Collector: filepath.Join(dir, "ccusage"), CollectorConfig: filepath.Join(dir, "ccusage.json"), Sources: []Source{a}}
	state := State{Destination: Destination(c), Entries: map[string]Entry{"pi/2026-01-02": {Revision: 7, Digest: "unchanged", Pending: &device.Snapshot{Revision: 7}}}}
	Save(filepath.Join(dir, "config.json"), c)
	Save(filepath.Join(dir, "state.json"), state)
	calls := 0
	choose := func(Config) ([]Source, error) { calls++; return []Source{b}, nil }
	if err := ReconfigureSources(dir, choose); err == nil || calls != 0 {
		t.Fatalf("pending should block before prompting: %v", err)
	}
	e := state.Entries["pi/2026-01-02"]
	e.Pending = nil
	state.Entries["pi/2026-01-02"] = e
	Save(filepath.Join(dir, "state.json"), state)
	before, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if err := ReconfigureSources(dir, choose); err != nil {
		t.Fatal(err)
	}
	var saved Config
	Load(filepath.Join(dir, "config.json"), &saved)
	if !reflect.DeepEqual(saved.Sources, []Source{b}) || Destination(saved) != Destination(c) {
		t.Fatal(saved)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("revisions/outbox changed")
	}
	// A newly discovered root must not enter the saved selection.
	sourceDir(t, dir, ".pi/agent/sessions")
	Load(filepath.Join(dir, "config.json"), &saved)
	if !reflect.DeepEqual(saved.Sources, []Source{b}) {
		t.Fatal(saved)
	}
	if err := ReconfigureSources(dir, func(Config) ([]Source, error) { return []Source{}, nil }); err != nil {
		t.Fatal(err)
	}
	Load(filepath.Join(dir, "config.json"), &saved)
	if len(saved.Sources) != 0 {
		t.Fatal("empty selection not persisted")
	}
	if err := Run(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "no approved sources") {
		t.Fatalf("empty consent must disable collection: %v", err)
	}
}
func TestEmptyMissingUnreadableAndChangedRootsFailClosed(t *testing.T) {
	dir := t.TempDir()
	p := sourceDir(t, dir, "sessions")
	if err := ValidateSources(nil); err == nil {
		t.Fatal("empty selection")
	}
	if err := ValidateSources([]Source{{"pi", filepath.Join(dir, "missing")}}); err == nil {
		t.Fatal("missing selection")
	}
	if err := os.Chmod(p, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(p, 0700)
	if err := ValidateSources([]Source{{"pi", p}}); err == nil {
		t.Fatal("unreadable selection")
	}
	if err := Run(context.Background(), dir); err == nil {
		t.Fatal("unconfigured run")
	}
}
func TestMergeSnapshotsAndOverflow(t *testing.T) {
	snapshots, err := ParseDaily([]byte(fixture), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	merged, err := mergeSnapshots(append(snapshots, snapshots...))
	if err != nil || len(merged) != 1 || merged[0].Totals.Input != 200 || merged[0].TotalTokens != 280 || merged[0].Models[0].Input != 180 {
		t.Fatalf("%+v %v", merged, err)
	}
	snapshots[0].TotalTokens = math.MaxInt64
	if _, err = mergeSnapshots(append(snapshots, snapshots...)); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestReconfigureUsesSyncLock(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = lockFile(lock); err != nil {
		t.Fatal(err)
	}
	if err = ReconfigureSources(dir, func(Config) ([]Source, error) { t.Fatal("prompted while locked"); return nil, nil }); err == nil || !strings.Contains(err.Error(), "another sync") {
		t.Fatal(err)
	}
}

func TestSourceBatches(t *testing.T) {
	home := t.TempDir()
	a := Source{"copilot", filepath.Join(home, "a")}
	b := Source{"copilot", filepath.Join(home, "b")}
	file := Source{"copilot-otel", filepath.Join(b.Path, "otel", "usage.jsonl")}
	pi := Source{"pi", filepath.Join(home, "pi")}
	pi2 := Source{"pi", filepath.Join(home, "pi2")}
	got := sourceBatches([]Source{a, b, file, pi, pi2})
	want := [][]Source{{a, pi, pi2}, {b}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	if got := sourceBatches([]Source{file}); !reflect.DeepEqual(got, [][]Source{{file}}) {
		t.Fatal(got)
	}
}

func TestCollectorEnvironmentExplicitExclusions(t *testing.T) {
	t.Setenv("PI_AGENT_DIR", "/unapproved")
	t.Setenv("COPILOT_OTEL_FILE_EXPORTER_PATH", "/unapproved")
	t.Setenv("SECRET_TEST_VALUE", "must-not-inherit")
	env := collectorEnvironment([]Source{{"pi", "/approved"}}, "/empty")
	values := map[string]string{}
	for _, v := range env {
		p := strings.SplitN(v, "=", 2)
		values[p[0]] = p[1]
	}
	for agent, spec := range sourceSpecs {
		want := "/empty"
		if agent == "pi" {
			want = "/approved"
		}
		if values[spec.env] != want {
			t.Fatalf("%s was not explicit: %+v", agent, values)
		}
	}
	if _, ok := values["SECRET_TEST_VALUE"]; ok {
		t.Fatal("inherited environment")
	}
}
func TestDiscoverConfigPathFieldsOnly(t *testing.T) {
	dir := t.TempDir()
	pi := sourceDir(t, dir, "custom/sessions")
	path := filepath.Join(dir, "ccusage.json")
	Save(path, map[string]any{"pi": map[string]any{"defaults": map[string]any{"piPath": pi}}, "token": "not-a-path"})
	sources, warnings := DiscoverConfigSources(dir, []string{path})
	if len(warnings) != 0 || !reflect.DeepEqual(sources, []Source{{"pi", pi}}) {
		t.Fatalf("%+v %+v", sources, warnings)
	}
}

// Opt-in real pinned collector test uses synthetic files only; no user usage is
// read or uploaded. Run DEVICE_USAGE_TEST_COLLECTOR=/absolute/ccusage go test ...
func TestPinnedCollectorSelection(t *testing.T) {
	collector := os.Getenv("DEVICE_USAGE_TEST_COLLECTOR")
	if collector == "" {
		t.Skip("set DEVICE_USAGE_TEST_COLLECTOR for pinned binary integration")
	}
	dir := t.TempDir()
	a := sourceDir(t, dir, "approved")
	b := sourceDir(t, dir, "unapproved")
	home := sourceDir(t, dir, "home")
	makePi := func(root, model string, n int) {
		raw := map[string]any{"type": "message", "timestamp": "2026-01-02T00:00:00.000Z", "message": map[string]any{"role": "assistant", "model": model, "usage": map[string]int{"input": n, "output": 2}}}
		data, _ := json.Marshal(raw)
		if err := os.WriteFile(filepath.Join(root, "session.jsonl"), append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	makePi(a, "approved-model", 10)
	makePi(b, "forbidden-model", 900)
	defaultPi := sourceDir(t, home, ".pi/agent/sessions")
	makePi(defaultPi, "forbidden-default", 999)
	// A different unselected agent with real synthetic usage must also stay out.
	openclaw := sourceDir(t, home, ".openclaw/agents/main/sessions")
	makePi(openclaw, "forbidden-agent", 999)
	t.Setenv("HOME", home)
	t.Setenv("PI_AGENT_DIR", b)
	t.Setenv("OPENCLAW_DIR", filepath.Join(home, ".openclaw"))
	c := Config{Collector: collector, CollectorConfig: filepath.Join(dir, "ccusage.json"), Sources: []Source{{"pi", a}}}
	Save(c.CollectorConfig, map[string]any{"pi": map[string]any{"defaults": map[string]any{"piPath": b}}})
	for i := 0; i < 2; i++ {
		got, err := Collect(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Agent != "pi" || got[0].Totals.Input != 10 {
			t.Fatalf("unexpected data: %+v", got)
		}
	}
	c.Sources = append(c.Sources, Source{"pi", b})
	got, err := Collect(context.Background(), c)
	if err != nil || len(got) != 1 || got[0].Totals.Input != 910 {
		t.Fatalf("multi-root: %+v %v", got, err)
	}
	c.Sources = nil
	if _, err = Collect(context.Background(), c); err == nil {
		t.Fatal("empty selection collected")
	}
	c.Sources = []Source{{"pi", filepath.Join(dir, "missing")}}
	if _, err = Collect(context.Background(), c); err == nil {
		t.Fatal("missing selection collected")
	}
}
