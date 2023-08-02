// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !linux && !darwin
// +build !linux,!darwin

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/gohai"

import (
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata/gohai"
	"go.uber.org/zap"
)

// GetProcessesPayload currently just a stub.
func NewProcessesPayload(_ string, _ *zap.Logger) *gohai.ProcessesPayload {
	// unimplemented for misc platforms.
	return &gohai.ProcessesPayload{}
}
