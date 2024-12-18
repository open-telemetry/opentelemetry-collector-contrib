// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/ecs/internal/encoder/log"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"
)

// Encoder allows encoding log records into the appropriate format for Elasticsearch indexation
type Encoder interface {
	EncodeLog(pcommon.Resource, string, plog.LogRecord, pcommon.InstrumentationScope, string) ([]byte, error)
}

// New returns the appropriate encoder based on the mapping mode
func New(dedot bool, mode mapping.Mode) Encoder {
	switch mode {
	case mapping.ModeECS:
		return &ecsEncoder{dedot}
	case mapping.ModeOTel:
		return &otelEncoder{dedot}
	case mapping.ModeBodyMap:
		return &bodyMapEncoder{}
	default:
		return &defaultEncoder{dedot, mode}
	}
}
