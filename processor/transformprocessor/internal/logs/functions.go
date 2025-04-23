// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func LogFunctions(additionalLogFuncs []ottl.Factory[ottllog.TransformContext]) map[string]ottl.Factory[ottllog.TransformContext] {
	// No logs-only functions yet.
	logFunctions := ottlfuncs.StandardFuncs[ottllog.TransformContext]()
	for _, fn := range additionalLogFuncs {
		logFunctions[fn.Name()] = fn
	}
	return logFunctions
}
