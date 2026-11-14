// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xtransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/xtransformprocessor"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/factoryoptions"
)

// WithProfileFunctions will override the default OTTL profile context functions with the provided profileFunctions in the resulting processor.
// Subsequent uses of WithProfileFunctions will merge the provided profileFunctions with the previously registered functions.
func WithProfileFunctions(profileFunctions []ottl.Factory[*ottlprofile.TransformContext]) transformprocessor.FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultProfileFunctionsOverridden {
			options.ProfileFunctions = map[string]ottl.Factory[*ottlprofile.TransformContext]{}
			options.DefaultProfileFunctionsOverridden = true
		}
		options.ProfileFunctions = factoryoptions.MergeFunctionsToMap(options.ProfileFunctions, profileFunctions)
	}
}
