// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"
)

var errHostnameOrIPNotFound = errors.New("hostname/ip not found in attributes")

type dnsLookupProcessor struct {
	config       *Config
	resolver     resolver.Resolver
	processPairs []processPair
	logger       *zap.Logger
}

// processPair holds a context ID and a function to process DNS lookups
type processPair struct {
	ContextID contextID
	ProcessFn func(ctx context.Context, pMap pcommon.Map) error
}

func newDNSLookupProcessor(config *Config, logger *zap.Logger) (*dnsLookupProcessor, error) {
	// TODO: share the cache/chain resolver across signals for each component.ID
	dnsResolver, err := createResolverChain(config, logger)
	if err != nil {
		return nil, err
	}

	dp := &dnsLookupProcessor{
		logger:   logger,
		config:   config,
		resolver: dnsResolver,
	}

	dp.processPairs = dp.createProcessPairs()

	return dp, nil
}

// createResolverChain creates a chain of resolvers based on the provided configuration.
// Returns an error if no resolvers are configured or if any of the resolvers fail to initialize.
func createResolverChain(config *Config, logger *zap.Logger) (resolver.Resolver, error) {
	if len(config.Hostfiles) > 0 {
		hostFileResolver, err := resolver.NewHostFileResolver(config.Hostfiles, logger)
		if err != nil {
			return nil, err
		}

		// TODO: replace with actual chain resolver implementation
		return hostFileResolver, nil
	}

	// TODO: replace with actual chain resolver implementation
	return resolver.NewNoOpResolver(), nil
}

// createProcessPairs creates a list of processPair based on the configuration.
func (dp *dnsLookupProcessor) createProcessPairs() []processPair {
	if (dp.config.Resolve != nil && dp.config.Reverse != nil) &&
		(dp.config.Resolve.Context == dp.config.Reverse.Context) {
		return []processPair{
			{
				ContextID: dp.config.Resolve.Context,
				ProcessFn: dp.processResolveReverseLookup,
			},
		}
	}

	var processPairs []processPair

	if dp.config.Resolve != nil {
		processPairs = append(processPairs, processPair{
			ContextID: dp.config.Resolve.Context,
			ProcessFn: dp.processResolveLookup,
		})
	}

	if dp.config.Reverse != nil {
		processPairs = append(processPairs, processPair{
			ContextID: dp.config.Reverse.Context,
			ProcessFn: dp.processReverseLookup,
		})
	}

	return processPairs
}

// processResolveReverseLookup performs both DNS forward and reverse lookups on a set of attributes
func (dp *dnsLookupProcessor) processResolveReverseLookup(ctx context.Context, pMap pcommon.Map) error {
	resolveErr := dp.processResolveLookup(ctx, pMap)
	reverseErr := dp.processReverseLookup(ctx, pMap)

	return errors.Join(resolveErr, reverseErr)
}

// processResolveLookup finds the hostname from attributes and resolves it to an IP address
func (dp *dnsLookupProcessor) processResolveLookup(ctx context.Context, pMap pcommon.Map) error {
	return dp.processLookup(
		ctx,
		pMap,
		dp.config.Resolve,
		func(hostname string) (string, error) {
			return resolver.ValidateHostname(resolver.NormalizeHostname(hostname))
		},
		dp.resolver.Resolve,
	)
}

// processReverseLookup finds the IP from attributes and resolves it to a hostname
func (dp *dnsLookupProcessor) processReverseLookup(ctx context.Context, pMap pcommon.Map) error {
	return dp.processLookup(
		ctx,
		pMap,
		dp.config.Reverse,
		resolver.ValidateIP,
		dp.resolver.Reverse,
	)
}

func (dp *dnsLookupProcessor) processLookup(
	ctx context.Context,
	pMap pcommon.Map,
	config *lookupConfig,
	validateFn func(string) (string, error),
	lookupFn func(context.Context, string) ([]string, error),
) error {
	source, err := strFromAttributes(config.SourceAttributes, pMap, validateFn)

	// no hostname/IP found in attributes
	if source == "" || err != nil {
		return nil
	}

	results, err := lookupFn(ctx, source)
	if err != nil {
		return err
	}

	// Successfully resolved with content. Save the results to attribute
	if len(results) > 0 {
		slice := pMap.PutEmptySlice(config.TargetAttribute)
		for _, res := range results {
			slice.AppendEmpty().SetStr(res)
		}
	}
	return nil
}

// strFromAttributes returns the first IP/hostname from the given attributes.
// It uses validateFn to check the format. If no valid IP/hostname is found, it returns an error.
func strFromAttributes(attributes []string, pMap pcommon.Map, validateFn func(string) (string, error)) (string, error) {
	lastErr := errHostnameOrIPNotFound

	for _, attr := range attributes {
		if val, found := pMap.Get(attr); found {
			parsedStr, err := validateFn(val.Str())
			if err == nil {
				return parsedStr, nil
			}

			lastErr = err
		}
	}

	return "", lastErr
}

func (dp *dnsLookupProcessor) processMetrics(_ context.Context, ms pmetric.Metrics) (pmetric.Metrics, error) {
	return ms, nil
}

func (dp *dnsLookupProcessor) processTraces(_ context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	return ts, nil
}
