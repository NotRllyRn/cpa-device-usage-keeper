//go:build !windows

package deviceclient

import (
	"os"
	"syscall"
)

func lockFile(f *os.File) error         { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
func replaceFile(from, to string) error { return os.Rename(from, to) }
func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func secureDirectory(string) error      { return nil }
func privateMode(info os.FileInfo) bool { return info.Mode().Perm()&0077 == 0 }
