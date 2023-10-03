// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func LogFunctions() map[string]ottl.Factory[ottllog.TransformContext] {
	// No logs-only functions yet.
	return ottlfuncs.StandardFuncs[ottllog.TransformContext]()
}
