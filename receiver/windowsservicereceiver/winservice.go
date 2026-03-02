// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type winService struct {
	mgr    *serviceManager
	name   string
	handle *mgr.Service

	status svc.Status
	config configEx
}

func updateService(m *serviceManager, sname string) (*winService, error) {
	s, err := m.openService(sname)
	if err != nil {
		return nil, err
	}
	return &winService{
		mgr:    m,
		name:   sname,
		handle: s,
	}, nil
}

func (ws *winService) updateStatus() error {
	if ws.handle == nil {
		return nil
	}
	st, err := ws.handle.Query()
	if err != nil {
		return err
	}
	ws.status = st
	return nil
}

func (ws *winService) updateConfig() error {
	if ws.handle == nil {
		return nil
	}
	cfg, err := ws.handle.Config()
	if err != nil {
		return err
	}
	ws.config = configEx{
		StartType:        StartType(cfg.StartType),
		DelayedAutoStart: cfg.DelayedAutoStart,
	}
	return nil
}

func (ws *winService) close() error {
	if ws.handle == nil {
		return nil
	}
	err := ws.handle.Close()
	ws.handle = nil
	return err
}
