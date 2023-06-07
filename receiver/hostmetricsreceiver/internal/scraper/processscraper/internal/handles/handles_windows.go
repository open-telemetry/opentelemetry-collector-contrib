// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package handles

import (
	"errors"
	"fmt"

	"github.com/yusufpapurcu/wmi"
)

func NewManager() Manager {
	// return &handleCountManager{processInfoGetter: &ntQuerySystemInfoGetter{}}
	return &wmiHandleCountManager{}
}

var (
	ErrNoHandleCounts          = errors.New("no handle counts are currently registered")
	ErrNoHandleCountForProcess = errors.New("no handle count for process")
)

type wmiHandleCountManager struct {
	handleCounts map[int64]uint32
}

type Win32_Process struct {
	ProcessID   int64
	HandleCount uint32
}

func (m *wmiHandleCountManager) Refresh() error {
	handleCounts, err := queryProcessHandleCounts()
	if err != nil {
		return err
	}

	newHandleCounts := make(map[int64]uint32, len(handleCounts))
	for _, p := range handleCounts {
		newHandleCounts[p.ProcessID] = p.HandleCount
	}
	m.handleCounts = newHandleCounts
	return nil
}

func (m *wmiHandleCountManager) GetProcessHandleCount(pid int64) (uint32, error) {
	if len(m.handleCounts) == 0 {
		return 0, ErrNoHandleCounts
	}
	handleCount, ok := m.handleCounts[pid]
	if !ok {
		return 0, fmt.Errorf("%w %d", ErrNoHandleCountForProcess, pid)
	}
	return handleCount, nil
}

func queryProcessHandleCounts() ([]Win32_Process, error) {
	handleCounts := []Win32_Process{}
	// Creates query `get-wmiobject -query "select ProcessId, HandleCount from Win32_Process"`
	// based on reflection of Win32_Process type.
	q := wmi.CreateQuery(&handleCounts, "")
	err := wmi.Query(q, &handleCounts)
	return handleCounts, err
}
