package main

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func TestRuntimeLifecycle(t *testing.T) {
	shutdown()
	defer shutdown()
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	config := []byte(fmt.Sprintf("listen: 127.0.0.1:0\ndata_dir: %q\n", dir))
	if err := configure(config); err != nil {
		t.Fatal(err)
	}
	runtime.Lock()
	address := runtime.server.Address()
	first := runtime.server
	runtime.Unlock()
	if err := configure(config); err != nil {
		t.Fatal(err)
	}
	if runtime.server != first {
		t.Fatal("same config replaced server")
	}
	if err := configure([]byte(fmt.Sprintf("listen: 127.0.0.1:9999\ndata_dir: %q\n", dir))); err == nil {
		t.Fatal("live config changed")
	}
	shutdown()
	shutdown()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal("listener not released", err)
	}
	listener.Close()
	if err := configure(config); err != nil {
		t.Fatal("storage lock not released", err)
	}
}
