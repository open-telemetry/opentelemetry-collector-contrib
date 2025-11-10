// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package customottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/customottl"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func LogFuncs() map[string]ottl.Factory[ottllog.TransformContext] {
	return commonFuncs[ottllog.TransformContext]()
}

func commonFuncs[K any]() map[string]ottl.Factory[K] {
	return ottlfuncs.StandardFuncs[K]()
}

