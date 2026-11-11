package device

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestListenerAuthAndLifecycle(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := Start(Config{Listen: "127.0.0.1:0", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	admin, err := os.ReadFile(filepath.Join(dir, "admin-token"))
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, token string, body []byte) (int, []byte) {
		t.Helper()
		req, _ := http.NewRequest(method, "http://"+s.Address()+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, raw
	}
	if code, _ := call("GET", "/v1/history", "", nil); code != 401 {
		t.Fatal(code)
	}
	code, raw := call("POST", "/v1/devices", string(bytes.TrimSpace(admin)), []byte(`{"name":"test-device"}`))
	if code != 201 {
		t.Fatalf("%d %s", code, raw)
	}
	var enrollment struct{ ID, Token string }
	json.Unmarshal(raw, &enrollment)
	if code, _ = call("GET", "/v1/history", enrollment.Token, nil); code != 401 {
		t.Fatal("device can read history")
	}
	payload, _ := json.Marshal(sample())
	if code, raw = call("POST", "/v1/snapshots", enrollment.Token, payload); code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	if code, _ = call("POST", "/v1/snapshots", string(bytes.TrimSpace(admin)), payload); code != 401 {
		t.Fatal("admin can upload")
	}
	if code, _ = call("POST", "/v1/snapshots", enrollment.Token, bytes.Repeat([]byte("x"), 65537)); code != 400 {
		t.Fatal(code)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = http.Get("http://" + s.Address() + "/healthz"); err == nil {
		t.Fatal("listener still open")
	}
}
