// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

type State uint32

const (
	StateStopped         State = 1
	StateStartPending    State = 2
	StateStopPending     State = 3
	StateRunning         State = 4
	StateContinuePending State = 5
	StatePausePending    State = 6
	StatePaused          State = 7
)

type StartType uint32

const (
	StartBoot      StartType = 0
	StartSystem    StartType = 1
	StartAutomatic StartType = 2
	StartManual    StartType = 3
	StartDisabled  StartType = 4
)

type configEx struct {
	StartType        StartType
	DelayedAutoStart bool
}

type serviceManager struct {
	svcmgr *mgr.Mgr
}

func (sm *serviceManager) connect() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	sm.svcmgr = m
	return nil
}

func (sm *serviceManager) disconnect() error {
	if sm.svcmgr != nil {
		return sm.svcmgr.Disconnect()
	}
	return nil
}

func (sm *serviceManager) listServices() ([]string, error) {
	if sm.svcmgr == nil {
		return []string{}, nil
	}
	return sm.svcmgr.ListServices()
}

func (sm *serviceManager) openService(name string) (*mgr.Service, error) {
	if sm.svcmgr == nil {
		return nil, windows.ERROR_INVALID_HANDLE
	}
	if name == "" {
		return nil, windows.ERROR_INVALID_PARAMETER
	}
	return sm.svcmgr.OpenService(name)
}
