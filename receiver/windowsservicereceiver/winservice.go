// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver

import (
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type winService struct {
	mgr    *serviceManager
	name   string
	handle *mgr.Service

	status svc.Status
	config ConfigEx
}

func getService(m *serviceManager, sname string) (*winService, error) {
	m.targetService = sname
	s, err := m.openService()
	if err != nil {
		return nil, err
	}
	return &winService{
		mgr:    m,
		name:   sname,
		handle: s,
	}, nil
}

func (ws *winService) getStatus() error {
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

func (ws *winService) getConfig() error {
	if ws.handle == nil {
		return nil
	}
	cfg, err := ws.handle.Config()
	if err != nil {
		return err
	}
	ws.config = ConfigEx{
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
