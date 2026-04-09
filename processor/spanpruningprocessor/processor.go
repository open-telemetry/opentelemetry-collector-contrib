// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type spanPruningProcessor struct{}

func newSpanPruningProcessor() *spanPruningProcessor {
	return &spanPruningProcessor{}
}

func (*spanPruningProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	return td, nil
}
