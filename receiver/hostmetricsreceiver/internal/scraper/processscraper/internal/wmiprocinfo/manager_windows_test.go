// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package wmiprocinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/wmiprocinfo"

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCountManager(t *testing.T) {
	testInfos := map[int64]*wmiProcInfo{
		1: {handleCount: 3, ppid: 10},
		2: {handleCount: 5, ppid: 20},
	}
	m := deterministicManagerWithInfo(testInfos)

	require.NoError(t, m.Refresh())

	count, err := m.GetProcessHandleCount(1)
	assert.NoError(t, err)
	assert.Equal(t, uint32(3), count)

	ppid, err := m.GetProcessPpid(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), ppid)

	count, err = m.GetProcessHandleCount(2)
	assert.NoError(t, err)
	assert.Equal(t, uint32(5), count)

	ppid, err = m.GetProcessPpid(2)
	assert.NoError(t, err)
	assert.Equal(t, int64(20), ppid)

	_, err = m.GetProcessHandleCount(3)
	assert.ErrorIs(t, errors.Unwrap(err), ErrProcessNotFound)
	assert.True(t, strings.Contains(err.Error(), "3"))
}

type mockQueryer struct {
	info map[int64]*wmiProcInfo
}

func (s mockQueryer) wmiProcessQuery() (map[int64]*wmiProcInfo, error) {
	return s.info, nil
}

func deterministicManagerWithInfo(info map[int64]*wmiProcInfo) *wmiProcInfoManager {
	return &wmiProcInfoManager{
		queryer: mockQueryer{info: info},
	}
}
