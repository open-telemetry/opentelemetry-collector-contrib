// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/gohai"

import (
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata/gohai"
	"go.uber.org/zap"
)

// NewPayload returns an empty gohai payload since aix is not supported.
func NewPayload(logger *zap.Logger) gohai.Payload {
	payload := gohai.NewEmpty()
	payload.Gohai.Gohai = newGohai(logger)
	return payload
}

func newGohai(logger *zap.Logger) *gohai.Gohai {
	logger.Info("Using noop gohai implementation for windows/arm64/aix since it is not supported")
	return new(gohai.Gohai)
}
