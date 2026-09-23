// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	xprofilefuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func ResourceFunctions() map[string]ottl.Factory[*ottlresource.TransformContext] {
	return xprofilefuncs.WithProfileConverters(ottlfuncs.StandardFuncs[*ottlresource.TransformContext]())
}

func ScopeFunctions() map[string]ottl.Factory[*ottlscope.TransformContext] {
	return xprofilefuncs.WithProfileConverters(ottlfuncs.StandardFuncs[*ottlscope.TransformContext]())
}
