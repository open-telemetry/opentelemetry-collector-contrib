// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"golang.org/x/sys/windows/svc/mgr"
)

// represents the service struct before we have converted it into a metric
type ServiceStatus struct {
	ServiceStatus uint32
	StartType     uint32
	service       *mgr.Service
}

func GetService(sname string) (*ServiceStatus, error) {
	m, err := SCConnect()
	defer m.Disconnect()
	if err != nil {
		return nil, err
	}

	service, err := m.OpenService(sname)
	if err != nil {
		return nil, err
	}

	s := ServiceStatus{
		service: service,
	}

	// populate metric fields
	if err = s.getStatus(); err != nil {
		return nil, err
	}

	if err = s.getConfig(); err != nil {
		return nil, err
	}

	return &s, nil
}

// populates fields from service status query
func (s *ServiceStatus) getStatus() error {
	st, err := s.service.Query()
	if err != nil {
		return err
	}
	s.ServiceStatus = uint32(st.State)
	return nil
}

// popualtes fields from service config query
func (s *ServiceStatus) getConfig() error {
	c, err := s.service.Config()
	if err != nil {
		return err
	}

	s.StartType = c.StartType
	return nil
}
