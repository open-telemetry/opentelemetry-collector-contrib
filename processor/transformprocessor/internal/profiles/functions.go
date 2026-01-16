// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/profiles"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func ProfileFunctions() map[string]ottl.Factory[ottlprofile.TransformContext] {
	// No profiles-only functions yet.
	return ottlfuncs.StandardFuncs[ottlprofile.TransformContext]()
}
