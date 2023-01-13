package gofs

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var addr string

func TestCreate_NoRootDir(t *testing.T) {
	opts := Opts{}
	fs, err := Create(opts)
	assert.Nil(t, fs)
	assert.NotNil(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to read root dir")
	}
}

func TestCreate_InvalidAddr(t *testing.T) {
	opts := Opts{
		RootDir: os.TempDir(),
	}
	fs, err := Create(opts)
	assert.Nil(t, fs)
	assert.NotNil(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "port not specified")
	}
}
