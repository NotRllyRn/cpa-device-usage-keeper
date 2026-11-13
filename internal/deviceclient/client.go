// Package deviceclient uploads sanitized ccusage aggregates with a durable outbox.
package deviceclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"time"

	device "cpa-usage-keeper/internal/devicereport"
)

const CollectorVersion = "20.0.43"

type Config struct {
	URL             string   `json:"url"`
	Token           string   `json:"token"`
	Collector       string   `json:"collector"`
	CollectorConfig string   `json:"collector_config"`
	Sources         []Source `json:"sources"`
}
type Entry struct {
	Revision int64            `json:"revision"`
	Digest   string           `json:"digest"`
	Pending  *device.Snapshot `json:"pending,omitempty"`
}

func Destination(c Config) string {
	sum := sha256.Sum256([]byte(strings.TrimRight(c.URL, "/") + "\x00" + c.Token))
	return hex.EncodeToString(sum[:])
}

type State struct {
	Destination string           `json:"destination"`
	Entries     map[string]Entry `json:"entries"`
	LastSuccess time.Time        `json:"last_success"`
}

func PrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || !privateMode(info) {
		return errors.New("state directory must be private (0700)")
	}
	return secureDirectory(dir)
}
func Save(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = replaceFile(f.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}
func Load(path string, v any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || !privateMode(info) {
		return errors.New("state file must be private")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
func ValidateConfig(c Config) error {
	u, err := url.Parse(c.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("server URL must be HTTPS without credentials, query or fragment")
	}
	if len(c.Token) != 64 {
		return errors.New("invalid upload token")
	}
	if _, err := hex.DecodeString(c.Token); err != nil {
		return errors.New("invalid upload token")
	}
	if !filepath.IsAbs(c.Collector) || !filepath.IsAbs(c.CollectorConfig) {
		return errors.New("collector and config paths must be absolute")
	}
	return nil
}

// Capped output prevents a faulty collector exhausting uploader memory. Raw
// stderr is deliberately discarded: it can contain local paths or log content.
type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("collector output limit exceeded")
	}
	return b.Buffer.Write(p)
}
func collectBatch(ctx context.Context, c Config) ([]device.Snapshot, error) {
	if err := ValidateSources(c.Sources); err != nil {
		return nil, err
	}
	// A new empty home/config on every invocation prevents all implicit source
	// and named-store discovery. Never use the user's editable ccusage config.
	empty, err := os.MkdirTemp(filepath.Dir(c.CollectorConfig), ".collector-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(empty)
	if err = os.Mkdir(filepath.Join(empty, "projects"), 0700); err != nil {
		return nil, err
	}
	config := filepath.Join(empty, "ccusage.json")
	if err = Save(config, map[string]any{}); err != nil {
		return nil, err
	}
	env := collectorEnvironment(c.Sources, empty)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	version := exec.CommandContext(ctx, c.Collector, "--version")
	version.Env, version.Dir = env, empty
	var out cappedBuffer
	out.limit = 1024
	version.Stdout = &out
	if version.Run() != nil || strings.TrimSpace(out.String()) != "ccusage "+CollectorVersion {
		return nil, errors.New("collector version mismatch")
	}
	cmd := exec.CommandContext(ctx, c.Collector, "daily", "--by-agent", "--json", "--timezone", "UTC", "--offline", "--config", config)
	cmd.Env, cmd.Dir = env, empty
	out = cappedBuffer{limit: 32 * 1024 * 1024}
	cmd.Stdout = &out
	if cmd.Run() != nil {
		return nil, errors.New("collector failed; raw output withheld for privacy")
	}
	snapshots, err := ParseDaily(out.Bytes(), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, s := range c.Sources {
		agent := s.Agent
		if agent == "copilot-otel" {
			agent = "copilot"
		}
		allowed[agent] = true
	}
	for _, s := range snapshots {
		if !allowed[s.Agent] {
			return nil, errors.New("collector returned an unselected agent")
		}
	}
	return snapshots, nil
}
func Run(ctx context.Context, dir string) error { return runSync(ctx, dir, false) }

// Drain only uploads the existing outbox. It is explicit because those payloads
// can contain data from roots the user intends to deselect.
func Drain(ctx context.Context, dir string) error { return runSync(ctx, dir, true) }

func runSync(ctx context.Context, dir string, drainOnly bool) error {
	if err := PrivateDir(dir); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = lockFile(lock); err != nil {
		return errors.New("another sync is running")
	}
	var c Config
	if err = Load(filepath.Join(dir, "config.json"), &c); err != nil {
		return err
	}
	if err = ValidateConfig(c); err != nil {
		return err
	}
	var state State
	path := filepath.Join(dir, "state.json")
	if err = Load(path, &state); err != nil {
		return errors.New("state unavailable; restore the original state or enroll a new device in a fresh state directory; do not reset revisions")
	}
	if state.Destination != Destination(c) {
		return errors.New("destination changed; restore the original configuration or enroll a new device in a fresh state directory")
	}
	if state.Entries == nil {
		return errors.New("invalid state; do not reset revisions")
	}
	if !drainOnly {
		if err = ValidateSources(c.Sources); err != nil {
			return err
		}
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	// Always acknowledge previously persisted payloads before collecting again.
	flush := func() error {
		keys := make([]string, 0, len(state.Entries))
		for key := range state.Entries {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entry := state.Entries[key]
			if entry.Pending == nil {
				continue
			}
			if err := upload(ctx, client, c, *entry.Pending); err != nil {
				return err
			}
			entry.Pending = nil
			state.Entries[key] = entry
			if err := Save(path, state); err != nil {
				return err
			}
		}
		return nil
	}
	if err = flush(); err != nil {
		return err
	}
	if drainOnly {
		return nil
	}
	snapshots, err := Collect(ctx, c)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		key := snapshot.Agent + "/" + snapshot.Day
		entry := state.Entries[key]
		raw, _ := json.Marshal(struct {
			Models []device.Model
			Totals device.Tokens
			Total  int64
		}{snapshot.Models, snapshot.Totals, snapshot.TotalTokens})
		sum := sha256.Sum256(raw)
		digest := hex.EncodeToString(sum[:])
		if digest == entry.Digest {
			continue
		}
		snapshot.Revision = entry.Revision + 1
		state.Entries[key] = Entry{Revision: snapshot.Revision, Digest: digest, Pending: &snapshot}
	}
	// Missing days are retained. Explicit smaller reports replace, never add.
	if err = Save(path, state); err != nil {
		return err
	}
	if err = flush(); err != nil {
		return err
	}
	state.LastSuccess = time.Now().UTC()
	return Save(path, state)
}
func upload(ctx context.Context, client *http.Client, c Config, s device.Snapshot) error {
	raw, _ := json.Marshal(s)
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.URL, "/")+"/v1/snapshots", bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4097))
			resp.Body.Close()
			if resp.StatusCode == 200 {
				var ack struct {
					Revision int64 `json:"revision"`
				}
				if readErr != nil || len(body) > 4096 || json.Unmarshal(body, &ack) != nil || ack.Revision != s.Revision {
					return errors.New("invalid acknowledgement; snapshot remains pending")
				}
				return nil
			}
			if resp.StatusCode < 500 && resp.StatusCode != 429 {
				return fmt.Errorf("upload rejected (%d); snapshot remains pending", resp.StatusCode)
			}
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}
	return errors.New("upload unavailable; snapshot remains pending")
}
