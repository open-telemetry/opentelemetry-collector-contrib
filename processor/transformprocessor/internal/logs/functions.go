// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

import (
	"maps"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	xprofilefuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"
)

func LogFunctions() map[string]ottl.Factory[*ottllog.TransformContext] {
	functions := xprofilefuncs.WithProfileConverters(ottlfuncs.StandardFuncs[*ottllog.TransformContext]())

	logFunctions := ottl.CreateFactoryMap(
		logparsingfuncs.NewParseCEFFactory(),
		logparsingfuncs.NewParseCLFFactory(),
		logparsingfuncs.NewParseELFFactory(),
		logparsingfuncs.NewParseLEEFFactory(),
	)

	maps.Copy(functions, logFunctions)

	return functions
}
