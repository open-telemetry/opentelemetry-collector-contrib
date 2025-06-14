// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//revive:disable:unused-parameter
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

/*
* Functions and structs which are used to interact with the Windows service api.
*
* Primary functions are connecting to the Service Control Manager (SCM) to gather service information on scrape.
*
* Docs may be found at https://learn.microsoft.com/en-us/windows/win32/services/services
* and "https://learn.microsoft.com/en-us/windows/win32/api/winsvc/"
**/

// service manager "client"
//
//nolint:unused
type serviceManager struct {
	handle windows.Handle // handle to SCM database
}

// get SCM database handle
//
//nolint:unused,unparam
func scmConnect() (*serviceManager, error) {
	var h windows.Handle
	return &serviceManager{
		h,
	}, nil
}

//nolint:unused
func (sm *serviceManager) disconnect() error {
	return nil
}

//nolint:unused
func (sm *serviceManager) listServices() ([]string, error) {
	var s []string
	return s, nil
}

//nolint:unused
func (sm *serviceManager) openService() (*mgr.Service, error) {
	return nil, nil
}
