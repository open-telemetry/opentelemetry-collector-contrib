// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package handlecount

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnsupportedManager(t *testing.T) {
	m := NewManager()

	err := m.Refresh()
	require.ErrorIs(t, err, ErrHandlesPlatformSupport)

	_, err = m.GetProcessHandleCount(0)
	assert.ErrorIs(t, err, ErrHandlesPlatformSupport)
}
