// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xtransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/xtransformprocessor"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
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

// WithResourceFunctions will override the default OTTL resource context functions with the provided resourceFunctions in the resulting processor.
// Subsequent uses of WithResourceFunctions will merge the provided resourceFunctions with the previously registered functions.
// The functions apply to statements parsed in the resource context. A statement without an explicit context
// can be inferred into a lower context, whose functions are not affected by this option.
func WithResourceFunctions(resourceFunctions []ottl.Factory[*ottlresource.TransformContext]) transformprocessor.FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultResourceFunctionsOverridden {
			options.ResourceFunctions = map[string]ottl.Factory[*ottlresource.TransformContext]{}
			options.DefaultResourceFunctionsOverridden = true
		}
		options.ResourceFunctions = factoryoptions.MergeFunctionsToMap(options.ResourceFunctions, resourceFunctions)
	}
}

// WithScopeFunctions will override the default OTTL scope context functions with the provided scopeFunctions in the resulting processor.
// Subsequent uses of WithScopeFunctions will merge the provided scopeFunctions with the previously registered functions.
// The functions apply to statements parsed in the scope context. A statement without an explicit context
// can be inferred into a lower context, whose functions are not affected by this option.
func WithScopeFunctions(scopeFunctions []ottl.Factory[*ottlscope.TransformContext]) transformprocessor.FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultScopeFunctionsOverridden {
			options.ScopeFunctions = map[string]ottl.Factory[*ottlscope.TransformContext]{}
			options.DefaultScopeFunctionsOverridden = true
		}
		options.ScopeFunctions = factoryoptions.MergeFunctionsToMap(options.ScopeFunctions, scopeFunctions)
	}
}
