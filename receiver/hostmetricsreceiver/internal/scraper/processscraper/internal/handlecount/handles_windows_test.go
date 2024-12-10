// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package handlecount

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCountManager(t *testing.T) {
	testInfos := map[int64]uint32{
		1: 3,
		2: 5,
	}
	m := deterministicManagerWithInfo(testInfos)

	require.NoError(t, m.Refresh())

	count, err := m.GetProcessHandleCount(1)
	assert.NoError(t, err)
	assert.Equal(t, uint32(3), count)

	count, err = m.GetProcessHandleCount(2)
	assert.NoError(t, err)
	assert.Equal(t, uint32(5), count)

	_, err = m.GetProcessHandleCount(3)
	assert.ErrorIs(t, errors.Unwrap(err), ErrNoHandleCountForProcess)
	assert.Contains(t, err.Error(), "3")
}

type mockQueryer struct {
	info map[int64]uint32
}

func (s mockQueryer) queryProcessHandleCounts() (map[int64]uint32, error) {
	return s.info, nil
}

func deterministicManagerWithInfo(info map[int64]uint32) *handleCountManager {
	return &handleCountManager{
		queryer: mockQueryer{info: info},
	}
}
