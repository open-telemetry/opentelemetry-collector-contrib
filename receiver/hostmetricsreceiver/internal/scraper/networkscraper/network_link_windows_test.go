// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package networkscraper

import (
	stdnet "net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLinkInfoFromWindowsInterface(t *testing.T) {
	info := linkInfoFromWindowsInterface(0, 100_000_000, 100_000_000)
	assert.Equal(t, 200_000_000.0, info.CapacityBitsPerSecond)
	assert.False(t, info.Loopback)

	info = linkInfoFromWindowsInterface(stdnet.FlagLoopback, 100_000_000, 100_000_000)
	assert.True(t, info.Loopback)
}
