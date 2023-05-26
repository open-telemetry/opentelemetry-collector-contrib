// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package handles

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows"
)

func TestHandleCountManager(t *testing.T) {
	testInfos := []*windows.SYSTEM_PROCESS_INFORMATION{
		{
			UniqueProcessID: 1,
			HandleCount:     3,
		},
		{
			UniqueProcessID: 2,
			HandleCount:     5,
		},
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

type mockSyscaller struct {
	info []*windows.SYSTEM_PROCESS_INFORMATION
}

func (s *mockSyscaller) getSystemProcessInformation() ([]*windows.SYSTEM_PROCESS_INFORMATION, error) {
	return s.info, nil
}

func deterministicManagerWithInfo(info []*windows.SYSTEM_PROCESS_INFORMATION) *handleCountManager {
	return &handleCountManager{
		processInfoGetter: &mockSyscaller{info: info},
	}
}
