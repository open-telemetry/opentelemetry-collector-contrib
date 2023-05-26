// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package handles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnsupportedManager(t *testing.T) {
	m := NewManager()

	err := m.Refresh()
	assert.ErrorIs(t, err, ErrHandlesPlatformSupport)

	_, err = m.GetProcessHandleCount(0)
	assert.ErrorIs(t, err, ErrHandlesPlatformSupport)
}
