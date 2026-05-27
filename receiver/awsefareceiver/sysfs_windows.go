// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package awsefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"

import (
	"errors"

	"go.uber.org/zap"
)

var errCounterNotAvailable = errors.New("counter not available")

// efaDevice represents a single EFA device with its port and counter values.
type efaDevice struct {
	name     string
	port     string
	eniID    string
	counters map[string]uint64
}

// sysFsReader is an interface for reading EFA data from sysfs, enabling testability.
type sysFsReader interface {
	EfaDataExists() (bool, error)
	ListDevices() ([]string, error)
	ListPorts(deviceName string) ([]string, error)
	ReadCounter(deviceName, port, counter string) (uint64, error)
	ReadGID(deviceName string) (string, error)
}

func newSysFsReader(_ string, _ *zap.Logger) sysFsReader {
	return &windowsSysFsReader{}
}

type windowsSysFsReader struct{}

func (r *windowsSysFsReader) EfaDataExists() (bool, error)         { return false, nil }
func (r *windowsSysFsReader) ListDevices() ([]string, error)       { return nil, nil }
func (r *windowsSysFsReader) ListPorts(_ string) ([]string, error) { return nil, nil }
func (r *windowsSysFsReader) ReadCounter(_, _, _ string) (uint64, error) {
	return 0, errCounterNotAvailable
}

func (r *windowsSysFsReader) ReadGID(_ string) (string, error) {
	return "", errors.New("EFA is not supported on Windows")
}
