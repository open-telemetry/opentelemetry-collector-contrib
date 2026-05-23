// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/gohai"

import (
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata/gohai"
	"go.uber.org/zap"
)

// GetProcessesPayload currently just a stub.
func NewProcessesPayload(_ string, logger *zap.Logger) *gohai.ProcessesPayload {
	// unimplemented for misc platforms.
	logger.Info("Using noop gohai implementation since this platform is not supported")
	return &gohai.ProcessesPayload{}
}
