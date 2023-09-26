// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/gohai"

import (
	"github.com/DataDog/gohai/processes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata/gohai"
	"go.uber.org/zap"
)

// NewProcessesPayload builds a payload of processes metadata collected from gohai.
func NewProcessesPayload(hostname string, logger *zap.Logger) *gohai.ProcessesPayload {
	// Get processes metadata from gohai
	proc, err := new(processes.Processes).Collect()
	if err != nil {
		logger.Warn("Failed to retrieve processes metadata", zap.Error(err))
		return nil
	}

	processesPayload := map[string]interface{}{
		"snaps": []interface{}{proc},
	}
	return &gohai.ProcessesPayload{
		Processes: processesPayload,
		Meta: map[string]string{
			"host": hostname,
		},
	}
}
