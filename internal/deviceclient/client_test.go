package deviceclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	device "cpa-usage-keeper/internal/devicereport"
)

const fixture = `{"daily":[{"period":"2026-01-02","agent":"all","agents":[{"agent":"pi","inputTokens":100,"outputTokens":20,"cacheReadTokens":10,"cacheCreationTokens":0,"totalTokens":140,"modelBreakdowns":[{"modelName":"[pi] example/model","inputTokens":90,"outputTokens":20,"cacheReadTokens":10,"cacheCreationTokens":0,"cost":999}],"projectPath":"/private/path","totalCost":999}]}],"totals":{}}`

func TestParseSanitizesAndRetainsTotals(t *testing.T) {
	snapshots, err := ParseDaily([]byte(fixture), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].TotalTokens != 140 || snapshots[0].Totals.Input != 100 || snapshots[0].Models[0].Input != 90 {
		t.Fatalf("%+v", snapshots)
	}
	raw, _ := json.Marshal(snapshots)
	var generic []map[string]any
	json.Unmarshal(raw, &generic)
	if _, ok := generic[0]["projectPath"]; ok {
		t.Fatal("leaked local path")
	}
	if _, err = ParseDaily([]byte(`{"daily":null}`), time.Now()); err == nil {
		t.Fatal("accepted null daily")
	}
}
func TestUploadAcknowledgementAndToken(t *testing.T) {
	mode := 200
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer upload-token" {
			t.Error("missing token")
		}
		w.WriteHeader(mode)
		w.Write([]byte(`{"revision":1}`))
	}))
	defer server.Close()
	c := Config{URL: server.URL, Token: "upload-token"}
	s := device.Snapshot{Revision: 1}
	if err := upload(context.Background(), server.Client(), c, s); err != nil {
		t.Fatal(err)
	}
	s.Revision = 2
	if err := upload(context.Background(), server.Client(), c, s); err == nil {
		t.Fatal("accepted wrong ack")
	}
	mode = 409
	if err := upload(context.Background(), server.Client(), c, s); err == nil {
		t.Fatal("accepted conflict")
	}
	if requests != 3 {
		t.Fatal("retried permanent error")
	}
}
func TestRunRefusesLostStateAndDestinationChange(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	c := Config{URL: "https://usage.example.com", Token: strings.Repeat("a", 64), Collector: "/nonexistent/ccusage", CollectorConfig: "/nonexistent/config"}
	if err := Save(filepath.Join(dir, "config.json"), c); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "state unavailable") {
		t.Fatalf("lost state: %v", err)
	}
	state := State{Destination: Destination(c), Entries: map[string]Entry{}}
	c.Token = strings.Repeat("b", 64)
	Save(filepath.Join(dir, "config.json"), c)
	Save(filepath.Join(dir, "state.json"), state)
	if err := Run(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "destination changed") {
		t.Fatalf("changed token: %v", err)
	}
	c.Token = strings.Repeat("a", 64)
	c.URL = "https://other.example.com"
	Save(filepath.Join(dir, "config.json"), c)
	if err := Run(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "destination changed") {
		t.Fatalf("changed URL: %v", err)
	}
}
func TestDurableState(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	path := filepath.Join(dir, "state.json")
	state := State{Entries: map[string]Entry{"pi/2026-01-02": {Revision: 4, Pending: &device.Snapshot{Revision: 4}}}}
	if err := Save(path, state); err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := Load(path, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Entries["pi/2026-01-02"].Pending.Revision != 4 {
		t.Fatal("pending payload lost")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}
