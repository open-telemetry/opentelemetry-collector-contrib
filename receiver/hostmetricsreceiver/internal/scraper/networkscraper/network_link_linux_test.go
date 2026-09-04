// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package networkscraper

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/stretchr/testify/assert"
)

func TestReadSpeedMbps(t *testing.T) {
	testCases := []struct {
		name     string
		contents string
		expected int64
	}{
		{name: "valid", contents: "1000\n", expected: 1000},
		{name: "negative unknown", contents: "-1\n", expected: 0},
		{name: "u16 unknown", contents: "65535\n", expected: 0},
		{name: "u32 unknown", contents: "4294967295\n", expected: 0},
		{name: "invalid", contents: "unknown\n", expected: 0},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "speed")
			assert.NoError(t, os.WriteFile(path, []byte(tt.contents), 0o600))
			assert.Equal(t, tt.expected, readSpeedMbps(path))
		})
	}
}

func TestReadDuplex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duplex")
	assert.NoError(t, os.WriteFile(path, []byte("full\n"), 0o600))
	assert.True(t, readDuplex(path))

	assert.NoError(t, os.WriteFile(path, []byte("half\n"), 0o600))
	assert.False(t, readDuplex(path))
}

func TestReadLoopback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flags")
	assert.NoError(t, os.WriteFile(path, []byte("0x9\n"), 0o600))
	assert.True(t, readLoopback(path))

	assert.NoError(t, os.WriteFile(path, []byte("0x1003\n"), 0o600))
	assert.False(t, readLoopback(path))
}

func TestReadNetworkLinkInfoUsesHostSysEnv(t *testing.T) {
	hostSys := t.TempDir()
	devicePath := filepath.Join(hostSys, "class", "net", "eth0")
	assert.NoError(t, os.MkdirAll(devicePath, 0o700))
	assert.NoError(t, os.WriteFile(filepath.Join(devicePath, "speed"), []byte("1000\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(devicePath, "duplex"), []byte("full\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(devicePath, "flags"), []byte("0x1003\n"), 0o600))

	ctx := context.WithValue(t.Context(), common.EnvKey, common.EnvMap{common.HostSysEnvKey: hostSys})
	info, err := readNetworkLinkInfo(ctx, "eth0")

	assert.NoError(t, err)
	assert.Equal(t, int64(1000), info.SpeedMbps)
	assert.True(t, info.FullDuplex)
	assert.False(t, info.Loopback)
}
