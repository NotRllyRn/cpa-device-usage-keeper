package device

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const sessionLifetime = 30 * 24 * time.Hour
const maxSessions = 128
const sessionCookie = "__Host-device_session"
const localSessionCookie = "device_session_local"

// A single global budget bounds work and memory even behind a proxy. Never key
// security decisions on client-supplied forwarding headers.
type loginBudget struct {
	mu       sync.Mutex
	start    time.Time
	attempts int
}

func (b *loginBudget) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.start.IsZero() || now.Sub(b.start) >= time.Minute {
		b.start = now
		b.attempts = 0
	}
	if b.attempts >= 10 {
		return false
	}
	b.attempts++
	return true
}

func sessionOrigin(value, listen string) (*url.URL, error) {
	if value == "" {
		return nil, nil
	} // Bearer-only installations remain supported.
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("session_origin must be an exact HTTP(S) origin without a trailing slash")
	}
	if u.Scheme == "http" {
		host, _, err := net.SplitHostPort(listen)
		ip := net.ParseIP(host)
		originIP := net.ParseIP(u.Hostname())
		if err != nil || ip == nil || !ip.IsLoopback() || (u.Hostname() != "localhost" && (originIP == nil || !originIP.IsLoopback())) {
			return nil, errors.New("HTTP sessions require a loopback listener and loopback session_origin")
		}
	}
	return u, nil
}

func (s *Server) sessionTransport(r *http.Request) bool {
	if s.origin == nil {
		return false
	}
	if s.origin.Scheme == "https" {
		return true
	} // Secure cookie even when TLS terminates at the proxy.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	return err == nil && ip != nil && ip.IsLoopback() && r.Host == s.origin.Host
}
func (s *Server) sessionMutation(r *http.Request) bool {
	return s.sessionTransport(r) && r.Header.Get("Origin") == s.origin.String() && r.Header.Get("Sec-Fetch-Site") != "cross-site"
}
func (s *Server) cookieName() string {
	if s.origin != nil && s.origin.Scheme == "http" {
		return localSessionCookie
	}
	return sessionCookie
}
func (s *Server) cookie(w http.ResponseWriter, token string, now time.Time) {
	c := &http.Cookie{Name: s.cookieName(), Value: token, Path: "/", HttpOnly: true, Secure: s.origin.Scheme == "https", SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime.Seconds()), Expires: now.Add(sessionLifetime)}
	if token == "" {
		c.MaxAge = -1
		c.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, c)
}
func (s *Server) sessionToken(r *http.Request) string {
	c, err := r.Cookie(s.cookieName())
	if err != nil || len(c.Value) != 64 {
		return ""
	}
	return c.Value
}
func (s *Server) validSession(r *http.Request) bool {
	if !s.sessionTransport(r) {
		return false
	}
	token := s.sessionToken(r)
	if token == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	var exists bool
	err := s.store.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash=? AND admin_hash=? AND expires_at>?)`, tokenHash(token), tokenHash(s.admin), time.Now().Unix()).Scan(&exists)
	return err == nil && exists
}
func (s *Server) createSession(ctx context.Context, now time.Time, previous string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// Table is capped on every insertion; all cleanup work is bounded by that cap.
	_, err = tx.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at<=? OR admin_hash<>? OR token_hash=?`, now.Unix(), tokenHash(s.admin), tokenHash(previous))
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash IN (SELECT token_hash FROM admin_sessions ORDER BY expires_at DESC, token_hash DESC LIMIT -1 OFFSET ?)`, maxSessions-1)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_sessions(token_hash,admin_hash,expires_at) VALUES(?,?,?)`, tokenHash(token), tokenHash(s.admin), now.Add(sessionLifetime).Unix())
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Server) serveSession(w http.ResponseWriter, r *http.Request) {
	if !s.sessionTransport(r) {
		fail(w, 503, "browser sessions require a configured session_origin")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !s.validSession(r) {
			fail(w, 401, "unauthorized")
			return
		}
		jsonReply(w, 200, map[string]bool{"authenticated": true})
	case http.MethodPost:
		if !s.sessionMutation(r) {
			fail(w, 403, "same-origin request required")
			return
		}
		if !s.login.allow(time.Now()) {
			w.Header().Set("Retry-After", "60")
			fail(w, 429, "too many login attempts; retry in one minute")
			return
		}
		var input struct {
			Token string `json:"token"`
		}
		if !decode(w, r, &input) {
			return
		}
		// Compare fixed-size digests so validation does not leak secret length.
		candidate, expected := tokenHash(input.Token), tokenHash(s.admin)
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) != 1 {
			fail(w, 401, "unauthorized")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		now := time.Now()
		token, err := s.createSession(ctx, now, s.sessionToken(r))
		if err != nil {
			fail(w, 503, "session storage unavailable")
			return
		}
		s.cookie(w, token, now)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		// Logout uses POST /v1/session/logout instead (see ServeHTTP).
		fail(w, 405, "method not allowed")
	default:
		fail(w, 405, "method not allowed")
	}
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !s.sessionMutation(r) {
		fail(w, 403, "same-origin request required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if token := s.sessionToken(r); token != "" {
		if _, err := s.store.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash=?`, tokenHash(token)); err != nil {
			fail(w, 503, "session storage unavailable; retry logout")
			return
		}
	}
	s.cookie(w, "", time.Now())
	w.WriteHeader(http.StatusNoContent)
}

func isRead(method string) bool { return method == http.MethodGet || method == http.MethodHead }
