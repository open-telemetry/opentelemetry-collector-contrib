// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"
import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

type Operator[T pmetric.Metric | plog.LogRecord | ptrace.Span | pcommon.Resource] interface {
	Do(ss migrate.StateSelector, data T) error
}
