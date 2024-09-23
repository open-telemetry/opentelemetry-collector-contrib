// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// represents the service struct before we have converted it into a metric
type Service struct {
	ServiceStatus uint32
	StartType     uint32
	Name          string
	service       *mgr.Service
}

func GetService(sname string) (*Service, error) {
	manager, err := mgr.Connect()
	defer manager.Disconnect()
	if err != nil {
		return nil, err
	}

	service, err := manager.OpenService(sname)
	if err != nil {
		return nil, err
	}

	s := Service{
		Name:    sname,
		service: service,
	}

	return &s, nil
}

// populates fields from service status query
//
// docs for the functions used are found here
// https://learn.microsoft.com/en-us/windows/win32/api/winsvc/nf-winsvc-queryservicestatusex
func (s *Service) getStatus() error {
	// we need to do this in order to get the size of the buffer we need to actually read the status
	var bytesNeeded uint32

	if err := windows.QueryServiceStatusEx(s.service.Handle, windows.SC_STATUS_PROCESS_INFO, nil, 0, &bytesNeeded); err != windows.ERROR_INSUFFICIENT_BUFFER {
		return err
	}

	// allocate our buffer with this fun little trick
	buf := make([]byte, bytesNeeded) // make an empty buffer of the required size
	// remember: unsafe.Pointer() is basically like void * in C, so this is probably pretty sketchy.
	lp := (*windows.SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buf[0]))

	// the second time around we know our required buffer (bytesRemaining is set to [bytes needed to read status] - [bytes of buffer supplied])
	if err := windows.QueryServiceStatusEx(s.service.Handle, windows.SC_STATUS_PROCESS_INFO, &buf[0], uint32(len(buf)), &bytesNeeded); err != nil {
		return err
	}

	s.ServiceStatus = lp.CurrentState
	return nil
}

// popualtes fields from service config query
func (s *Service) getConfig() error {
	c, err := s.service.Config()
	if err != nil {
		return err
	}

	s.StartType = c.StartType
	return nil
}
