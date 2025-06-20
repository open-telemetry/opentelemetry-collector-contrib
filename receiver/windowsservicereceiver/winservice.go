// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//revive:disable:unused-parameter
//go:build windows

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import "golang.org/x/sys/windows/svc/mgr"

/**
* Windows Service representation and associated functions. These handle
* interacting with the SCM and martialing service information returned by the
* windows api calls.
**/

// receiver representation of a service
//
//nolint:unused
type winService struct {
	service       *mgr.Service
	serviceStatus uint32
	startType     uint32
}

//nolint:unused
func getService(mgr *serviceManager, sname string) (*winService, error) {
	return &winService{}, nil
}

//nolint:unused
func (w *winService) getStatus() error {
	return nil
}

//nolint:unused
func (w *winService) getConfig() error {
	return nil
}
