// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver

import (
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

type Manager struct {
	Handle windows.Handle
}

func SCConnect() (*Manager, error) {
	var s *uint16

	h, err := windows.OpenSCManager(s, nil, windows.GENERIC_READ)
	if err != nil {
		return nil, err
	}

	return &Manager{
		Handle: h,
	}, nil
}

func (m *Manager) Disconnect() error {
	return windows.CloseServiceHandle(m.Handle)
}

func (m *Manager) ListServices() ([]string, error) {
	// TODO: implement ListServices with lowered permissions to return a list of services from service control manager db
	var s []string

	return s, nil
}

func (m *Manager) OpenService(name string) (*mgr.Service, error) {
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}

	h, err := windows.OpenService(m.Handle, ptr, windows.GENERIC_READ)
	if err != nil {
		return nil, err
	}

	return &mgr.Service{Name: name, Handle: h}, nil
}
