// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package wmiprocinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/wmiprocinfo"

import (
	"errors"
	"fmt"

	"github.com/yusufpapurcu/wmi"
)

func NewManager() Manager {
	return &wmiProcInfoManager{queryer: wmiProcessQueryer{}}
}

var (
	ErrNoProcesses     = errors.New("no process info is currently registered")
	ErrProcessNotFound = errors.New("process info not found")
)

type wmiProcInfo struct {
	handleCount uint32
	ppid        int64
}

type wmiQueryer interface {
	wmiProcessQuery() (map[int64]*wmiProcInfo, error)
}

type wmiProcInfoManager struct {
	queryer   wmiQueryer
	processes map[int64]*wmiProcInfo
}

func (m *wmiProcInfoManager) Refresh() error {
	processes, err := m.queryer.wmiProcessQuery()
	if err != nil {
		return err
	}
	m.processes = processes
	return nil
}

func (m *wmiProcInfoManager) getProcessInfo(pid int64) (*wmiProcInfo, error) {
	if len(m.processes) == 0 {
		return nil, ErrNoProcesses
	}
	procInfo, ok := m.processes[pid]
	if !ok {
		return nil, fmt.Errorf("%w for %d", ErrProcessNotFound, pid)
	}
	return procInfo, nil
}

func (m *wmiProcInfoManager) GetProcessHandleCount(pid int64) (uint32, error) {
	procInfo, err := m.getProcessInfo(pid)
	if err != nil {
		return 0, err
	}
	return procInfo.handleCount, nil
}

func (m *wmiProcInfoManager) GetProcessPpid(pid int64) (int64, error) {
	procInfo, err := m.getProcessInfo(pid)
	if err != nil {
		return 0, err
	}
	return procInfo.ppid, nil
}

type wmiProcessQueryer struct{}

//revive:disable-next-line:var-naming
type Win32_Process struct {
	ProcessID       int64
	ParentProcessID int64
	HandleCount     uint32
}

func (wmiProcessQueryer) wmiProcessQuery() (map[int64]*wmiProcInfo, error) {
	processes := []Win32_Process{}
	// Based on reflection of Win32_Process type, this creates the following query:
	// `get-wmiobject -query "select ProcessId, ParentProcessId, HandleCount from Win32_Process"`
	q := wmi.CreateQuery(&processes, "")
	err := wmi.Query(q, &processes)
	if err != nil {
		return nil, err
	}

	procInfos := make(map[int64]*wmiProcInfo, len(processes))
	for _, p := range processes {
		procInfos[p.ProcessID] = &wmiProcInfo{
			handleCount: p.HandleCount,
			ppid:        p.ParentProcessID,
		}
	}
	return procInfos, nil
}
