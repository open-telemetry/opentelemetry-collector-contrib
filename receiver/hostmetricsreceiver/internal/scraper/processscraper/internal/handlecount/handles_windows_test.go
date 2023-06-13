// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package handlecount

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleCountManager(t *testing.T) {
	testInfos := map[int64]uint32{
		1: 3,
		2: 5,
	}
	m := deterministicManagerWithInfo(testInfos)

	m.Refresh()

	count, err := m.GetProcessHandleCount(1)
	assert.NoError(t, err)
	assert.Equal(t, count, uint32(3))

	count, err = m.GetProcessHandleCount(2)
	assert.NoError(t, err)
	assert.Equal(t, count, uint32(5))

	_, err = m.GetProcessHandleCount(3)
	assert.ErrorIs(t, errors.Unwrap(err), ErrNoHandleCountForProcess)
	assert.True(t, strings.Contains(err.Error(), "3"))
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
