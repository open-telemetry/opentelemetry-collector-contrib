// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package handles // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/handles"

import (
	"errors"
	"fmt"

	"github.com/yusufpapurcu/wmi"
)

func NewManager() Manager {
	return &handleCountManager{queryer: wmiHandleCountQueryer{}}
}

var (
	ErrNoHandleCounts          = errors.New("no handle counts are currently registered")
	ErrNoHandleCountForProcess = errors.New("no handle count for process")
)

type handleCountQueryer interface {
	queryProcessHandleCounts() (map[int64]uint32, error)
}

type handleCountManager struct {
	queryer      handleCountQueryer
	handleCounts map[int64]uint32
}

func (m *handleCountManager) Refresh() error {
	handleCounts, err := m.queryer.queryProcessHandleCounts()
	if err != nil {
		return err
	}
	m.handleCounts = handleCounts
	return nil
}

func (m *handleCountManager) GetProcessHandleCount(pid int64) (uint32, error) {
	if len(m.handleCounts) == 0 {
		return 0, ErrNoHandleCounts
	}
	handleCount, ok := m.handleCounts[pid]
	if !ok {
		return 0, fmt.Errorf("%w %d", ErrNoHandleCountForProcess, pid)
	}
	return handleCount, nil
}

type wmiHandleCountQueryer struct{}

type Win32_Process struct {
	ProcessID   int64
	HandleCount uint32
}

func (wmiHandleCountQueryer) queryProcessHandleCounts() (map[int64]uint32, error) {
	handleCounts := []Win32_Process{}
	// Creates query `get-wmiobject -query "select ProcessId, HandleCount from Win32_Process"`
	// based on reflection of Win32_Process type.
	q := wmi.CreateQuery(&handleCounts, "")
	err := wmi.Query(q, &handleCounts)
	if err != nil {
		return nil, err
	}

	newHandleCounts := make(map[int64]uint32, len(handleCounts))
	for _, p := range handleCounts {
		newHandleCounts[p.ProcessID] = p.HandleCount
	}
	return newHandleCounts, nil
}
