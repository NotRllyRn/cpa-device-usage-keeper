package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"golang.org/x/term"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"cpa-usage-keeper/internal/deviceclient"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "device-sync:", err)
		os.Exit(1)
	}
}
func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".local", "state", "device-usage-keeper")
	if value := os.Getenv("DEVICE_USAGE_STATE"); value != "" {
		dir = value
	}
	if !filepath.IsAbs(dir) {
		return errors.New("state path must be absolute")
	}
	command := "sync"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "version":
		fmt.Println("device-sync 0.1.0-dev; ccusage " + deviceclient.CollectorVersion)
		return nil
	case "sync":
		return deviceclient.Run(context.Background(), dir)
	case "status":
		if err := deviceclient.PrivateDir(dir); err != nil {
			return err
		}
		var state deviceclient.State
		if err := deviceclient.Load(filepath.Join(dir, "state.json"), &state); err != nil {
			return err
		}
		pending := 0
		for _, entry := range state.Entries {
			if entry.Pending != nil {
				pending++
			}
		}
		fmt.Printf("Snapshots: %d\nPending: %d\nLast success: %s\n", len(state.Entries), pending, state.LastSuccess)
		return nil
	case "configure":
		if err := deviceclient.PrivateDir(dir); err != nil {
			return err
		}
		path := filepath.Join(dir, "config.json")
		for _, name := range []string{"config.json", "state.json"} {
			if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
				return errors.New("configuration/state already exists or is inaccessible; preserve it, use update or a fresh directory with a newly enrolled device")
			}
		}
		terminal := "/dev/tty"
		if runtime.GOOS == "windows" {
			terminal = "CONIN$"
		}
		tty, err := os.OpenFile(terminal, os.O_RDWR, 0)
		if err != nil {
			return errors.New("interactive terminal required")
		}
		defer tty.Close()
		reader := bufio.NewReader(tty)
		prompt := func(label string) (string, error) {
			fmt.Fprint(os.Stderr, label)
			v, e := reader.ReadString('\n')
			return strings.TrimSpace(v), e
		}
		endpoint, err := prompt("HTTPS server URL: ")
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stderr, "Upload-only token (hidden): ")
		secret, err := term.ReadPassword(int(tty.Fd()))
		token := strings.TrimSpace(string(secret))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		collector := filepath.Join(home, ".local", "lib", "device-usage-keeper", "ccusage")
		if runtime.GOOS == "windows" {
			collector += ".exe"
		}
		config := filepath.Join(dir, "ccusage.json")
		c := deviceclient.Config{URL: endpoint, Token: token, Collector: collector, CollectorConfig: config}
		if err = deviceclient.ValidateConfig(c); err != nil {
			return err
		}
		if _, err = os.Lstat(config); os.IsNotExist(err) {
			if err = deviceclient.Save(config, map[string]any{"pi": map[string]any{"defaults": map[string]any{"piPath": strings.Join([]string{filepath.Join(home, ".pi", "agent", "sessions"), filepath.Join(home, ".config", "pi-t3", "agent", "sessions")}, ",")}}}); err != nil {
				return err
			}
		}
		if err = deviceclient.Save(filepath.Join(dir, "state.json"), deviceclient.State{Destination: deviceclient.Destination(c), Entries: map[string]deviceclient.Entry{}}); err != nil {
			return err
		}
		return deviceclient.Save(path, c)
	default:
		return errors.New("usage: device-sync configure|sync|status|version")
	}
}
