package device

import (
	"os"
	"os/exec"
	"testing"
)

// Optional local integration: requires prebuilt device UI and a Playwright
// installation. Uses synthetic state only; never reads user collector files.
func TestBrowser(t *testing.T) {
	module := os.Getenv("DEVICE_TEST_PLAYWRIGHT")
	if module == "" {
		t.Skip("set DEVICE_TEST_PLAYWRIGHT to a Playwright module path")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := Start(Config{Listen: "127.0.0.1:0", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, token, err := s.store.Enroll("test-laptop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.store.Apply(token, sample()); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "../../scripts/test-browser.mjs")
	cmd.Env = append(os.Environ(), "DEVICE_TEST_URL=http://"+s.Address(), "DEVICE_TEST_TOKEN="+s.admin, "DEVICE_TEST_PLAYWRIGHT="+module)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("browser: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
