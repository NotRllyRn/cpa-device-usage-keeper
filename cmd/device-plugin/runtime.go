package main

import (
	"bytes"
	"errors"
	"net/url"
	"sync"

	"cpa-usage-keeper/internal/device"
	"gopkg.in/yaml.v3"
)

var runtime struct {
	sync.Mutex
	server       *device.Server
	config       device.Config
	dashboardURL string
}

// A bind/storage change requires a CPA restart: never close a healthy instance
// before a replacement configuration is known to work, or run two DB owners.
func configure(raw []byte) error {
	runtime.Lock()
	defer runtime.Unlock()
	var cfg struct {
		Listen       string `yaml:"listen"`
		DataDir      string `yaml:"data_dir"`
		DashboardURL string `yaml:"dashboard_url"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	} // unconfigured discovery is inert
	if err := decoder.Decode(&cfg); err != nil {
		return errors.New("invalid plugin configuration")
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8788"
	}
	if cfg.DashboardURL != "" {
		u, err := url.Parse(cfg.DashboardURL)
		if err != nil || u.Host == "" || u.User != nil || u.Scheme != "https" || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("dashboard_url must be an HTTPS URL without credentials, query or fragment")
		}
	}
	next := device.Config{Listen: cfg.Listen, DataDir: cfg.DataDir}
	if runtime.server != nil {
		if runtime.config != next {
			return errors.New("listen/data_dir changes require CPA restart")
		}
		runtime.dashboardURL = cfg.DashboardURL
		return nil
	}
	server, err := device.Start(next)
	if err != nil {
		return errors.New("cannot start device listener; check bind address and private data directory")
	}
	runtime.server = server
	runtime.config = next
	runtime.dashboardURL = cfg.DashboardURL
	return nil
}
func shutdown() {
	runtime.Lock()
	defer runtime.Unlock()
	if runtime.server != nil {
		_ = runtime.server.Close()
		runtime.server = nil
		runtime.config = device.Config{}
	}
}
