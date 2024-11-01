// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package customottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func SpanFuncs() map[string]ottl.Factory[ottlspan.TransformContext] {
	common := commonFuncs[ottlspan.TransformContext]()
	adjustedCountFactory := NewAdjustedCountFactory()
	common[adjustedCountFactory.Name()] = adjustedCountFactory
	return common
}

func DatapointFuncs() map[string]ottl.Factory[ottldatapoint.TransformContext] {
	return commonFuncs[ottldatapoint.TransformContext]()
}

func LogFuncs() map[string]ottl.Factory[ottllog.TransformContext] {
	return commonFuncs[ottllog.TransformContext]()
}

func commonFuncs[K any]() map[string]ottl.Factory[K] {
	getFactory := NewGetFactory[K]()
	standard := ottlfuncs.StandardFuncs[K]()
	standard[getFactory.Name()] = getFactory
	return standard
}
