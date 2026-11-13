package deviceclient

import (
	"errors"
	"os"
	"path/filepath"
)

// ReconfigureSources holds the same lock as sync and changes only the allowlist.
// Outbox revisions, destination, digests, and acknowledgements stay untouched.
func ReconfigureSources(dir string, choose func(Config) ([]Source, error)) error {
	if err := PrivateDir(dir); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = lockFile(lock); err != nil {
		return errors.New("another sync or source selection is running")
	}
	var c Config
	if err = Load(filepath.Join(dir, "config.json"), &c); err != nil {
		return err
	}
	if err = ValidateConfig(c); err != nil {
		return err
	}
	var state State
	if err = Load(filepath.Join(dir, "state.json"), &state); err != nil {
		return err
	}
	if state.Destination != Destination(c) || state.Entries == nil {
		return errors.New("invalid state/destination; restore the original files, do not reset revisions")
	}
	for _, entry := range state.Entries {
		if entry.Pending != nil {
			return errors.New("pending uploads may contain deselected data; selection unchanged. Run device-sync drain to explicitly upload them, then retry sources. History already uploaded is not deleted")
		}
	}
	sources, err := choose(c)
	if err != nil {
		return err
	}
	if len(sources) > 0 {
		if err = ValidateSources(sources); err != nil {
			return err
		}
	}
	c.Sources = sources
	return Save(filepath.Join(dir, "config.json"), c)
}
