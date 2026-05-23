// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package lookupsource provides the public API for creating lookup sources.
//
// This package allows third-party developers to implement custom lookup sources
// that can be registered with the lookup processor.
//
// # Creating a Custom Source
//
// To create a custom lookup source:
//
//  1. Implement your lookup logic as a [LookupFunc]
//  2. Create a [SourceFactory] using [NewSourceFactory]
//  3. Register it with the processor using [WithSources]
//
// Example:
//
//	func NewMySourceFactory() lookupsource.SourceFactory {
//	    return lookupsource.NewSourceFactory(
//	        "mysource",
//	        func() lookupsource.SourceConfig { return &MyConfig{} },
//	        createMySource,
//	    )
//	}
//
//	func createMySource(ctx context.Context, set lookupsource.CreateSettings, cfg lookupsource.SourceConfig) (lookupsource.Source, error) {
//	    c := cfg.(*MyConfig)
//	    lookupFn := func(ctx context.Context, key string) (any, bool, error) {
//	        // Your lookup logic here
//	        return value, true, nil
//	    }
//	    return lookupsource.NewSource(lookupFn, func() string { return "mysource" }, nil, nil), nil
//	}
//
// # Registering with the Processor
//
//	import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"
//
//	factories.Processors[lookupprocessor.Type] = lookupprocessor.NewFactoryWithOptions(
//	    lookupprocessor.WithSources(mysource.NewFactory()),
//	)
package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
