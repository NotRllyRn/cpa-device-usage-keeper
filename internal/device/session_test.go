package device

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
)

const testOrigin = "https://devices.example.test"

func sessionServer(t *testing.T) (*Server, Config) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Listen: "127.0.0.1:0", DataDir: dir, SessionOrigin: testOrigin}
	s, err := Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, cfg
}
func sessionRequest(s *Server, method, path, origin, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, testOrigin+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func loginSession(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"token": s.admin})
	w := sessionRequest(s, "POST", "/v1/session", testOrigin, string(raw), nil)
	if w.Code != 204 {
		t.Fatalf("login %d: %s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}
func TestSessionPersistenceRotationExpiryAndLogout(t *testing.T) {
	s, cfg := sessionServer(t)
	cookie := loginSession(t, s)
	if cookie.Name != sessionCookie || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" || cookie.MaxAge != 2592000 || len(cookie.Value) != 64 || cookie.Value == s.admin {
		t.Fatalf("invalid session cookie attributes")
	}
	var stored, binding string
	if err := s.store.db.QueryRow(`SELECT token_hash,admin_hash FROM admin_sessions`).Scan(&stored, &binding); err != nil {
		t.Fatal(err)
	}
	if stored != tokenHash(cookie.Value) || binding != tokenHash(s.admin) || stored == cookie.Value {
		t.Fatal("session must store digests only")
	}
	check := func(path string, want int) {
		t.Helper()
		w := sessionRequest(s, "GET", path, "", "", cookie)
		if w.Code != want {
			t.Fatalf("%s got %d want %d", path, w.Code, want)
		}
	}
	check("/v1/session", 200)
	check("/v1/devices", 200)
	if w := sessionRequest(s, "POST", "/v1/snapshots", testOrigin, `{}`, cookie); w.Code != 401 {
		t.Fatal("cookie authorized device upload")
	}
	s.Close()
	var err error
	s, err = Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	check("/v1/session", 200)
	w := sessionRequest(s, "POST", "/v1/session/logout", testOrigin, "", cookie)
	if w.Code != 204 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout failed: %d", w.Code)
	}
	check("/v1/session", 401)
	cookie = loginSession(t, s)
	if _, err = s.store.db.Exec(`UPDATE admin_sessions SET expires_at=?`, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	check("/v1/session", 401)
	cookie = loginSession(t, s)
	s.Close()
	rotated, _ := randomToken()
	if err = os.WriteFile(filepath.Join(cfg.DataDir, "admin-token"), []byte(rotated+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	check("/v1/session", 401)
	cookie = loginSession(t, s)
	check("/v1/session", 200)
}
func TestSessionCSRFAndBearerSeparation(t *testing.T) {
	s, _ := sessionServer(t)
	cookie := loginSession(t, s)
	for _, origin := range []string{"", "null", "https://evil.test", "https://devices.example.test.evil.test", "http://devices.example.test"} {
		for _, path := range []string{"/v1/session", "/v1/session/logout", "/v1/devices"} {
			w := sessionRequest(s, "POST", path, origin, `{"name":"test"}`, cookie)
			if w.Code != 403 {
				t.Errorf("origin %q path %s: %d", origin, path, w.Code)
			}
		}
	}
	if w := sessionRequest(s, "POST", "/v1/devices", testOrigin, `{"name":"test"}`, cookie); w.Code != 201 {
		t.Fatalf("same origin mutation: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("POST", testOrigin+"/v1/devices", strings.NewReader(`{"name":"bearer"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+s.admin)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("bearer compatibility %d", w.Code)
	}
	r = httptest.NewRequest("POST", testOrigin+"/v1/session/logout", nil)
	r.AddCookie(cookie)
	r.Header.Set("Origin", testOrigin)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site metadata accepted")
	}
	r = httptest.NewRequest("GET", testOrigin+"/v1/devices", nil)
	r.AddCookie(cookie)
	r.Header.Set("Authorization", "Bearer invalid")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("invalid explicit bearer fell back to cookie")
	}
}
func TestSessionRateAndStorageBounds(t *testing.T) {
	s, _ := sessionServer(t)
	for i := 0; i < 10; i++ {
		if w := sessionRequest(s, "POST", "/v1/session", testOrigin, `{"token":"wrong"}`, nil); w.Code != 401 {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	if w := sessionRequest(s, "POST", "/v1/session", testOrigin, `{"token":"`+s.admin+`"}`, nil); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("login budget not enforced")
	}
	now := time.Now()
	var first string
	for i := 0; i < maxSessions+3; i++ {
		token, err := s.createSession(context.Background(), now.Add(time.Duration(i)*time.Second), "")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = token
		}
	}
	var count int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM admin_sessions`).Scan(&count); err != nil || count != maxSessions {
		t.Fatalf("cap: %d %v", count, err)
	}
	if s.validSession(func() *http.Request {
		r := httptest.NewRequest("GET", testOrigin+"/v1/session", nil)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: first})
		return r
	}()) {
		t.Fatal("oldest session not evicted")
	}
	b := loginBudget{}
	for i := 0; i < 10; i++ {
		if !b.allow(now) {
			t.Fatal("early rejection")
		}
	}
	if b.allow(now) || !b.allow(now.Add(time.Minute)) {
		t.Fatal("budget window")
	}
}
func TestSessionHTTPSBehindProxy(t *testing.T) {
	s, _ := sessionServer(t)
	r := httptest.NewRequest("POST", "http://backend.internal/v1/session", strings.NewReader(`{"token":"`+s.admin+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", testOrigin)
	r.Header.Set("X-Forwarded-Proto", "http")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if r.TLS != nil || w.Code != 204 || !w.Result().Cookies()[0].Secure {
		t.Fatal("TLS proxy must still issue Secure cookies")
	}
	first := w.Result().Cookies()[0]
	replacement := sessionRequest(s, "POST", "/v1/session", testOrigin, `{"token":"`+s.admin+`"}`, first)
	if replacement.Code != 204 || replacement.Result().Cookies()[0].Value == first.Value {
		t.Fatal("login must rotate opaque session")
	}
	if sessionRequest(s, "GET", "/v1/session", "", "", first).Code != 401 {
		t.Fatal("replaced session remains valid")
	}
}

func TestSessionOriginConfiguration(t *testing.T) {
	for _, v := range []string{"null", "https://user@host", "https://host/", "https://host?x=1", "http://example.test", "ftp://host"} {
		if _, err := sessionOrigin(v, "127.0.0.1:8788"); err == nil {
			t.Errorf("accepted %s", v)
		}
	}
	if _, err := sessionOrigin("http://localhost:8788", "0.0.0.0:8788"); err == nil {
		t.Fatal("non-loopback HTTP listener")
	}
	s, _ := sessionServer(t)
	s.origin, _ = sessionOrigin("http://127.0.0.1:8788", "127.0.0.1:8788")
	r := httptest.NewRequest("POST", "http://127.0.0.1:8788/v1/session", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Origin", "http://127.0.0.1:8788")
	if !s.sessionMutation(r) {
		t.Fatal("explicit local HTTP rejected")
	}
	w := httptest.NewRecorder()
	s.cookie(w, "opaque", time.Now())
	if w.Result().Cookies()[0].Secure || w.Result().Cookies()[0].Name != localSessionCookie {
		t.Fatal("local cookie")
	}
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	r.Header.Set("X-Forwarded-Proto", "https")
	if s.sessionTransport(r) {
		t.Fatal("trusted forwarded headers")
	}
	s.origin = nil
	if w := sessionRequest(s, "POST", "/v1/session", testOrigin, `{}`, nil); w.Code != 503 {
		t.Fatal("unconfigured cookies enabled")
	}
}
