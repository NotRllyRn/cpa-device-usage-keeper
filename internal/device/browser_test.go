package device

import (
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Optional local integration: requires prebuilt device UI and a Playwright
// installation. Uses synthetic state only; never reads user collector files.
func TestBrowser(t *testing.T) {
	module := os.Getenv("DEVICE_TEST_PLAYWRIGHT")
	if module == "" {
		t.Skip("set DEVICE_TEST_PLAYWRIGHT to a Playwright module path")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reserved.Addr().String()
	reserved.Close()
	s, err := Start(Config{Listen: address, DataDir: dir, SessionOrigin: "http://" + address})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, token, err := s.store.Enroll("test-laptop")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := sample()
	snapshot.Day = time.Now().UTC().Format("2006-01-02")
	snapshot.CollectedAt = time.Now().UTC()
	if _, err = s.store.Apply(token, snapshot); err != nil {
		t.Fatal(err)
	}
	script := "../../scripts/test-browser.mjs"
	if os.Getenv("DEVICE_COMPARE_UI") != "" {
		script = "../../scripts/compare-device-ui.mjs"
	}
	if os.Getenv("DEVICE_ANALYSIS_UI") != "" {
		script = "../../scripts/compare-device-analysis.mjs"
	}
	if os.Getenv("DEVICE_ANALYSIS_SCOPE") != "" {
		script = "../../scripts/test-analysis-scope.mjs"
	}
	if os.Getenv("DEVICE_MODEL_FAMILY") != "" {
		script = "../../scripts/test-model-family.mjs"
		// Real API fixtures: mixed families within one Pi/Hermes snapshot,
		// a custom agent, older unknown totals, and pricing independent names.
		if _, err := s.store.db.Exec(`DELETE FROM snapshots`); err != nil {
			t.Fatal(err)
		}
		_, second, err := s.store.Enroll("test-desktop")
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		for _, row := range []struct {
			token, agent string
			day          time.Time
			models       []Model
			extra        int64
		}{
			{token, "pi", now.AddDate(0, 0, -2), []Model{{Name: "[pi] gpt-5.4", Tokens: Tokens{Input: 1000}}, {Name: "[pi] claude-sonnet-4-5", Tokens: Tokens{Input: 2000}}, {Name: "[pi] MiniMax-M2.7", Tokens: Tokens{Input: 3000}}, {Name: "mystery", Tokens: Tokens{Input: 500}}}, 700},
			{second, "hermes", now.AddDate(0, 0, -1), []Model{{Name: "gpt-5.4", Tokens: Tokens{Input: 4000}}, {Name: "claude-opus-4-6", Tokens: Tokens{Input: 5000}}, {Name: "MiniMax-M2.5", Tokens: Tokens{Input: 6000}}}, 0},
			{token, "custom-worker", now, []Model{{Name: "openai/o3-mini", Tokens: Tokens{Input: 800}}}, 0},
			{token, "old", now.AddDate(0, 0, -30), []Model{}, 900},
		} {
			value := Snapshot{Agent: row.agent, Day: row.day.Format("2006-01-02"), Revision: 1, CollectedAt: now, Models: row.models, TotalTokens: row.extra}
			for _, m := range row.models {
				addTokens(&value.Totals, m.Tokens)
				value.TotalTokens += m.Input
			}
			if _, err := s.store.Apply(row.token, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("node", script)
	cmd.Env = append(os.Environ(), "DEVICE_TEST_URL=http://"+s.Address(), "DEVICE_TEST_TOKEN="+s.admin, "DEVICE_TEST_PLAYWRIGHT="+module)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("browser: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
