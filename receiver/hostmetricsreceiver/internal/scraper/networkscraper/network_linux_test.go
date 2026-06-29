// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadInterfaceSpeedMbps(t *testing.T) {
	sys := t.TempDir()
	write := func(device, speed string) {
		require.NoError(t, os.MkdirAll(filepath.Join(sys, "class", "net", device), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(sys, "class", "net", device, "speed"), []byte(speed), 0o644))
	}
	write("eth0", "1000\n")
	write("lo", "-1\n")
	write("bad", "notanumber\n")
	t.Setenv("HOST_SYS", sys)

	speed, ok := readInterfaceSpeedMbps("eth0")
	assert.True(t, ok)
	assert.Equal(t, int64(1000), speed)

	_, ok = readInterfaceSpeedMbps("lo")
	assert.False(t, ok, "down/virtual links report -1 and should be skipped")

	_, ok = readInterfaceSpeedMbps("bad")
	assert.False(t, ok, "unparseable speed should be skipped")

	_, ok = readInterfaceSpeedMbps("missing")
	assert.False(t, ok, "missing interface should be skipped")
}
