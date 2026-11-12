package device

import (
	"context"
	"cpa-usage-keeper/internal/pricing"
	"cpa-usage-keeper/internal/pricingmetadata"
	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"cpa-usage-keeper/web"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config deliberately separates upload listener auth from CPA credentials.
// Plain HTTP is intended behind a TLS-terminating reverse proxy.
type Config struct {
	Listen  string `json:"listen"`
	DataDir string `json:"data_dir"`
}
type Server struct {
	store    *Store
	http     *http.Server
	listener net.Listener
	admin    string
	slots    chan struct{}
	once     sync.Once
	closeErr error
}

func Start(cfg Config) (*Server, error) {
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8788"
	}
	if !filepath.IsAbs(cfg.DataDir) {
		return nil, errors.New("data_dir must be absolute")
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("data_dir must be a private directory (0700)")
	}
	adminPath := filepath.Join(cfg.DataDir, "admin-token")
	// Only generate a secret on first start; never return it through CPA or logs.
	f, err := os.OpenFile(adminPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		token, e := randomToken()
		if e == nil {
			_, e = f.WriteString(token + "\n")
		}
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e != nil {
			return nil, e
		}
		if closeErr != nil {
			return nil, closeErr
		}
	} else if !os.IsExist(err) {
		return nil, err
	}
	info, err = os.Lstat(adminPath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("admin-token must be a private regular file")
	}
	secret, err := os.ReadFile(adminPath)
	if err != nil {
		return nil, err
	}
	admin := strings.TrimSpace(string(secret))
	if len(admin) != 64 {
		return nil, errors.New("invalid admin-token file")
	}
	store, err := OpenStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		store.Close()
		return nil, err
	}
	s := &Server{store: store, listener: listener, admin: admin, slots: make(chan struct{}, 8)}
	s.http = &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
	go func() { _ = s.http.Serve(listener) }()
	return s, nil
}
func (s *Server) Address() string { return s.listener.Addr().String() }
func (s *Server) Close() error {
	s.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.closeErr = s.http.Shutdown(ctx)
		if s.closeErr != nil {
			_ = s.http.Close()
		}
		if err := s.store.Close(); s.closeErr == nil {
			s.closeErr = err
		}
	})
	return s.closeErr
}
func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(v, "Bearer ")
}
func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	jsonReply(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		fail(w, 415, "application/json required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		fail(w, 400, "invalid or oversized JSON")
		return false
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		fail(w, 400, "expected one JSON value")
		return false
	}
	return true
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// No CORS: browser mutations require explicit bearer authentication. Never
	// use ambient cookies or query-string tokens on this listener.
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		fail(w, 503, "busy; retry later")
		return
	}
	if r.Method == "GET" && r.URL.Path == "/healthz" {
		jsonReply(w, 200, map[string]string{"status": "ok"})
		return
	}
	if r.Method == "POST" && r.URL.Path == "/v1/snapshots" {
		token := bearer(r)
		if len(token) != 64 || !s.store.Authorized(token) {
			fail(w, 401, "unauthorized")
			return
		}
		var snapshot Snapshot
		if !decode(w, r, &snapshot) {
			return
		}
		if err := snapshot.Validate(time.Now()); err != nil {
			fail(w, 400, err.Error())
			return
		}
		replay, err := s.store.Apply(token, snapshot)
		switch {
		case errors.Is(err, ErrUnauthorized):
			fail(w, 401, "unauthorized")
		case errors.Is(err, ErrConflict):
			fail(w, 409, "revision conflict")
		case err != nil:
			fail(w, 500, "storage unavailable")
		default:
			jsonReply(w, 200, map[string]any{"revision": snapshot.Revision, "replay": replay})
		}
		return
	}
	if r.Method == "GET" && r.URL.Path == "/" {
		raw, err := fs.ReadFile(web.DeviceAssets(), "device.html")
		if err != nil {
			fail(w, 503, "dashboard build missing")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'")
		_, _ = w.Write(raw)
		return
	}
	if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/assets/") {
		http.FileServer(http.FS(web.DeviceAssets())).ServeHTTP(w, r)
		return
	}
	if subtle.ConstantTimeCompare([]byte(bearer(r)), []byte(s.admin)) != 1 {
		fail(w, 401, "unauthorized")
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/overview":
		q := r.URL.Query()
		for _, key := range []string{"start", "end"} {
			if value := q.Get(key); value != "" {
				if _, err := time.Parse("2006-01-02", value); err != nil {
					fail(w, 400, "invalid UTC date")
					return
				}
			}
		}
		if len(q.Get("device")) > 160 || len(q.Get("agent")) > 160 || q.Get("end") != "" && q.Get("start") > q.Get("end") {
			fail(w, 400, "invalid filters")
			return
		}
		overview, err := s.store.Overview(r.Context(), q.Get("device"), q.Get("agent"), q.Get("start"), q.Get("end"))
		if err != nil {
			fail(w, 503, "overview unavailable; narrow the range or retry")
			return
		}
		jsonReply(w, 200, overview)
	case r.Method == "POST" && r.URL.Path == "/v1/pricing/sync/apply":
		var input servicedto.PricingSyncApplyRequest
		if !decode(w, r, &input) {
			return
		}
		models, err := s.store.ApplyBasePrices(r.Context(), input)
		if err != nil {
			fail(w, 400, "invalid pricing sync")
			return
		}
		jsonReply(w, 200, servicedto.PricingSyncApplyResponse{Models: models})
	case r.Method == "GET" && r.URL.Path == "/v1/pricing/sync":
		source := r.URL.Query().Get("source")
		if _, err := pricingmetadata.SourceByID(source); err != nil {
			fail(w, 400, "unsupported pricing source")
			return
		}
		catalog, err := pricingmetadata.NewClient(&http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}).Fetch(r.Context(), source)
		if err != nil {
			fail(w, 502, "pricing source unavailable")
			return
		}
		models, err := s.store.PricingModelIdentifiers(r.Context())
		if err != nil {
			fail(w, 503, "model list unavailable")
			return
		}
		jsonReply(w, 200, service.MatchCompletePricingCatalog(models, catalog))
	case r.Method == "GET" && r.URL.Path == "/v1/pricing/models":
		prices, err := s.store.Prices()
		if err != nil {
			fail(w, 500, "storage unavailable")
			return
		}
		jsonReply(w, 200, prices)
	case r.Method == "PUT" && r.URL.Path == "/v1/pricing/models":
		var p pricing.ModelPricingConfig
		if !decode(w, r, &p) {
			return
		}
		if err := s.store.SavePrice(p); err != nil {
			fail(w, 400, "invalid pricing configuration")
			return
		}
		w.WriteHeader(204)
	case r.Method == "DELETE" && r.URL.Path == "/v1/pricing/models":
		if err := s.store.DeletePrice(r.URL.Query().Get("model")); err != nil {
			fail(w, 500, "storage unavailable")
			return
		}
		w.WriteHeader(204)
	case r.Method == "POST" && r.URL.Path == "/v1/devices":
		var input struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &input) {
			return
		}
		id, token, err := s.store.Enroll(input.Name)
		if err != nil {
			fail(w, 400, "could not enroll device")
			return
		}
		jsonReply(w, 201, map[string]string{"id": id, "token": token})
	case r.Method == "GET" && r.URL.Path == "/v1/devices":
		devices, err := s.store.Devices()
		if err != nil {
			fail(w, 500, "storage unavailable")
			return
		}
		jsonReply(w, 200, devices)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/devices/"):
		if err := s.store.Revoke(strings.TrimPrefix(r.URL.Path, "/v1/devices/")); err != nil {
			fail(w, 500, "storage unavailable")
			return
		}
		w.WriteHeader(204)
	case r.Method == "GET" && r.URL.Path == "/v1/history":
		offset := 0
		var err error
		if value := r.URL.Query().Get("after"); value != "" {
			offset, err = strconv.Atoi(value)
		}
		if err != nil || offset < 0 || offset > 10_000_000 {
			fail(w, 400, "invalid cursor")
			return
		}
		rows, err := s.store.History(offset)
		if err != nil {
			fail(w, 500, "storage unavailable")
			return
		}
		prices, err := s.store.Prices()
		if err != nil {
			fail(w, 500, "storage unavailable")
			return
		}
		byModel := map[string]*pricing.ModelPricingConfig{}
		for i := range prices {
			byModel[prices[i].Model] = &prices[i]
		}
		type pricedRecord struct {
			Record
			Estimates []Estimate `json:"estimates"`
		}
		result := make([]pricedRecord, 0, len(rows))
		for _, row := range rows {
			p := pricedRecord{Record: row, Estimates: []Estimate{}}
			for _, m := range row.Models {
				p.Estimates = append(p.Estimates, estimate(m, resolvePrice(m.Name, byModel)))
			}
			result = append(result, p)
		}
		jsonReply(w, 200, result)
	default:
		fail(w, 404, "not found")
	}
}
