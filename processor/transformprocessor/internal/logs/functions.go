// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

import (
	"maps"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func LogFunctions() map[string]ottl.Factory[*ottllog.TransformContext] {
	functions := ottlfuncs.StandardFuncs[*ottllog.TransformContext]()

	logFunctions := ottl.CreateFactoryMap(
		newParseLEEFFactory(),
	)

	maps.Copy(functions, logFunctions)

	return functions
}
