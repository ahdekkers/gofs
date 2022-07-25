package gofs

import (
	"fmt"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var addr string

func TestCreate_NoRootDir(t *testing.T) {
	opts := Opts{}
	err := Create(opts)
	assert.NotNil(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to read root dir")
	}
}

func TestCreate_InvalidAddr(t *testing.T) {
	opts := Opts{
		RootDir: os.TempDir(),
	}
	err := Create(opts)
	assert.NotNil(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "port not specified")
	}
}

func setupTestServer(t *testing.T) {
	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	opts := Opts{
		Addr:     "127.0.0.1",
		Port:     port,
		LogLevel: "TRACE",
		RootDir:  os.TempDir(),
	}
	err = Create(opts)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	addr = fmt.Sprintf("127.0.0.1:%d", port)
}
