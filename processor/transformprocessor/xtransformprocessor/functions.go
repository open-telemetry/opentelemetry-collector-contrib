// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xtransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/xtransformprocessor"

import (
	"maps"
	"slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/profiles"
)

// DefaultProfileFunctions returns the default OTTL functions available in the profile context.
func DefaultProfileFunctions() []ottl.Factory[*ottlprofile.TransformContext] {
	return slices.Collect(maps.Values(profiles.ProfileFunctions()))
}
