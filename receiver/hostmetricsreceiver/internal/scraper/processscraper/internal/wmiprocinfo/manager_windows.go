// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package wmiprocinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/wmiprocinfo"

import (
	"errors"
	"fmt"

	"github.com/yusufpapurcu/wmi"
)

// NewManager will create a new wmiProcInfoManager, constructing
// the WMI query based on the provided options.
func NewManager(queryOpts ...QueryOption) Manager {
	query, err := constructQueryString(queryOpts...)
	if err != nil {
		return nil
	}
	return &wmiProcInfoManager{
		querier: wmiProcessQuerier{
			query: query,
			client: &wmi.Client{
				AllowMissingFields: true,
			},
		},
	}
}

type QueryOption func(string) string

// WithHandleCount includes the HandleCount field in the WMI query.
func WithHandleCount(q string) string {
	return q + ", HandleCount"
}

// WithParentProcessId includes the ParentProcessId field in the WMI query.
func WithParentProcessId(q string) string {
	return q + ", ParentProcessId"
}

func constructQueryString(queryOpts ...QueryOption) (string, error) {
	queryStr := "SELECT ProcessId"
	queryOptSet := false

	for _, opt := range queryOpts {
		queryStr = opt(queryStr)
		queryOptSet = true
	}

	if !queryOptSet {
		return "", errors.New("no query options supplied")
	}

	return queryStr + " FROM Win32_Process", nil
}

var (
	ErrNoProcesses     = errors.New("no process info is currently registered")
	ErrProcessNotFound = errors.New("process info not found")
)

type wmiProcInfo struct {
	handleCount uint32
	ppid        int64
}

type wmiQuerier interface {
	wmiProcessQuery() (map[int64]*wmiProcInfo, error)
}

type wmiProcInfoManager struct {
	querier   wmiQuerier
	processes map[int64]*wmiProcInfo
}

func (m *wmiProcInfoManager) Refresh() error {
	processes, err := m.querier.wmiProcessQuery()
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

type wmiProcessQuerier struct {
	query  string
	client *wmi.Client
}

//revive:disable-next-line:var-naming
type Win32_Process struct {
	ProcessID       int64
	ParentProcessID int64
	HandleCount     uint32
}

func (q wmiProcessQuerier) wmiProcessQuery() (map[int64]*wmiProcInfo, error) {
	processes := []Win32_Process{}
	if err := q.client.Query(q.query, &processes); err != nil {
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
