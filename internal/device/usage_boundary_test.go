package device

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestUsageBoundary(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := Start(Config{Listen: "127.0.0.1:0", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	idA, a, _ := s.store.Enroll("first")
	idB, b, _ := s.store.Enroll("second")
	check := func(device, agent, want string) {
		t.Helper()
		got, err := s.store.EarliestUsageDay(context.Background(), device, agent, "")
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if got != nil {
				t.Fatal(*got)
			}
			return
		}
		if got == nil || *got != want {
			t.Fatalf("%s/%s: %v want %s", device, agent, got, want)
		}
	}
	check("", "", "")
	put := func(token, agent, day string) {
		t.Helper()
		snapshot := sample()
		snapshot.Agent = agent
		snapshot.Day = day
		if _, err := s.store.Apply(token, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	put(a, "pi", "2024-01-01")
	put(b, "pi", "2023-01-01")
	put(a, "codex", "2022-01-01")
	check("", "", "2022-01-01")
	check(idA, "pi", "2024-01-01")
	check(idB, "", "2023-01-01")
	check("", "pi", "2023-01-01")
	check(idB, "codex", "")
	check("missing", "", "")
	put(b, "pi", "2020-01-01")
	check(idB, "pi", "2020-01-01")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.store.EarliestUsageDay(ctx, "", "", ""); err == nil {
		t.Fatal("ignored cancellation")
	}
	for _, tc := range []struct {
		token, path string
		status      int
	}{
		{"", "/v1/usage-boundary", 401}, {a, "/v1/usage-boundary", 401}, {s.admin, "/v1/usage-boundary", 200}, {s.admin, "/v1/usage-boundary?agent=" + strings.Repeat("x", 161), 400},
	} {
		req, _ := http.NewRequest("GET", "http://"+s.Address()+tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != tc.status {
			t.Fatalf("%s: %d", tc.path, response.StatusCode)
		}
	}
}
