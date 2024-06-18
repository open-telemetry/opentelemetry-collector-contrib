// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package key // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"

import (
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

// Partitioner allows for switching our partitioning behavior
// when sending data to kinesis.
type Partitioner interface {
	Partition(v any) string
	Keyed() bool
}

type Randomized struct{}

func (p Randomized) Partition(_ any) string {
	return uuid.NewString()
}

func (p Randomized) Keyed() bool {
	return false
}

type TraceID struct{}

func (p TraceID) Partition(v any) string {
	if trace, ok := v.(ptrace.Traces); ok {
		traceKey := traceutil.TraceIDToHexOrEmptyString(trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
		if len(traceKey) != 0 {
			return traceKey
		}
	}

	return uuid.NewString()
}

func (p TraceID) Keyed() bool {
	return true
}
