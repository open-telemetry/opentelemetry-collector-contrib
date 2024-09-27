// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package wmiprocinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/wmiprocinfo"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnsupportedManager(t *testing.T) {
	m := NewManager()

	err := m.Refresh()
	assert.ErrorIs(t, err, ErrPlatformSupport)

	_, err = m.GetProcessHandleCount(0)
	assert.ErrorIs(t, err, ErrPlatformSupport)
}
