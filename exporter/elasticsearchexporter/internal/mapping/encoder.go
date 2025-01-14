// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mapping // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

// Encoder provider an interface for all mapping encoders
type Encoder interface {
	EncodeLog(pcommon.Resource, plog.ScopeLogs, plog.LogRecord) objmodel.Document
	EncodeSpan(ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span) objmodel.Document
}
