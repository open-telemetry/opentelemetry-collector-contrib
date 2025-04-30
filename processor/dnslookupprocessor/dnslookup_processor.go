// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"
)

var (
	errUnknownContextID     = errors.New("unknown attribute context")
	errHostnameOrIPNotFound = errors.New("hostname/ip not found in attributes")
)

type dnsLookupProcessor struct {
	config       *Config
	resolver     resolver.Resolver
	processPairs []ProcessPair
	logger       *zap.Logger
}

// ProcessPair holds a context ID and a function to process DNS lookups
type ProcessPair struct {
	ContextID ContextID
	ProcessFn func(ctx context.Context, pMap pcommon.Map) error
}

func newDNSLookupProcessor(config *Config, logger *zap.Logger) (*dnsLookupProcessor, error) {
	dnsResolver, err := createResolverChain(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver chain: %w", err)
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
// The resolution order is cache -> chain(hostfiles -> nameservers -> system resolver).
// Returns either a chain resolver or a cache resolver if cache is enabled.
// Returns an error if no resolvers are configured or if any of the resolvers fail to initialize.
func createResolverChain(config *Config, logger *zap.Logger) (resolver.Resolver, error) {
	var chainResolver resolver.Resolver
	var resolvers []resolver.Resolver

	if len(config.Hostfiles) > 0 {
		hostFileResolver, err := resolver.NewHostFileResolver(
			config.Hostfiles,
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create hostfile resolver: %w", err)
		}

		resolvers = append(resolvers, hostFileResolver)
	}

	if len(config.Nameservers) > 0 {
		nameserverResolver, err := resolver.NewNameserverResolver(
			config.Nameservers,
			time.Duration(config.Timeout*float64(time.Second)),
			config.MaxRetries,
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create nameserver resolver: %w", err)
		}

		resolvers = append(resolvers, nameserverResolver)
	}

	if config.EnableSystemResolver {
		systemResolver := resolver.NewSystemResolver(
			time.Duration(config.Timeout*float64(time.Second)),
			config.MaxRetries,
			logger,
		)
		resolvers = append(resolvers, systemResolver)
	}

	if len(resolvers) == 0 {
		return nil, fmt.Errorf("no DNS resolver configuration available: either hostfile, nameserver, or system resolver must be enabled")
	}

	chainResolver = resolver.NewChainResolver(resolvers, logger)

	if config.HitCacheSize > 0 || config.MissCacheSize > 0 {
		cacheResolver, err := resolver.NewCacheResolver(
			chainResolver,
			config.HitCacheSize,
			time.Duration(config.HitCacheTTL)*time.Second,
			config.MissCacheSize,
			time.Duration(config.MissCacheTTL)*time.Second,
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache resolver: %w", err)
		}

		return cacheResolver, nil
	}

	return chainResolver, nil
}

// createProcessPairs creates a list of ProcessPair based on the configuration.
func (dp *dnsLookupProcessor) createProcessPairs() []ProcessPair {
	if dp.config.Resolve.Enabled && dp.config.Reverse.Enabled &&
		(dp.config.Resolve.Context == dp.config.Reverse.Context) {
		return []ProcessPair{
			{
				ContextID: dp.config.Resolve.Context,
				ProcessFn: dp.processDNSLookup,
			},
		}
	}

	var processPairs []ProcessPair

	if dp.config.Resolve.Enabled {
		processPairs = append(processPairs, ProcessPair{
			ContextID: dp.config.Resolve.Context,
			ProcessFn: dp.processResolveLookup,
		})
	}

	if dp.config.Reverse.Enabled {
		processPairs = append(processPairs, ProcessPair{
			ContextID: dp.config.Reverse.Context,
			ProcessFn: dp.processReverseLookup,
		})
	}

	return processPairs
}

// processDNSLookup performs both DNS forward and reverse lookups on a set of attributes
func (dp *dnsLookupProcessor) processDNSLookup(ctx context.Context, pMap pcommon.Map) error {
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
			hostname = resolver.NormalizeHostname(hostname)
			return resolver.ParseHostname(hostname)
		},
		dp.resolver.Resolve,
		resolver.LogKeyHostname,
	)
}

// processReverseLookup finds the IP from attributes and resolves it to a hostname
func (dp *dnsLookupProcessor) processReverseLookup(ctx context.Context, pMap pcommon.Map) error {
	return dp.processLookup(
		ctx,
		pMap,
		dp.config.Reverse,
		resolver.ParseIP,
		dp.resolver.Reverse,
		resolver.LogKeyIP,
	)
}

func (dp *dnsLookupProcessor) processLookup(
	ctx context.Context,
	pMap pcommon.Map,
	config LookupConfig,
	parseFn func(string) (string, error),
	lookupFn func(context.Context, string) (string, error),
	logKey string,
) error {
	target, err := targetStrFromAttributes(config.Attributes, pMap, parseFn)
	if err != nil {
		dp.logger.Debug(err.Error())
		return nil
	}
	if target == "" {
		dp.logger.Debug(fmt.Sprintf("Skip lookup for empty %s", logKey))
		return nil
	}

	// Found a valid target. Try to resolve it
	result, err := lookupFn(ctx, target)
	if err != nil {
		dp.logger.Debug(fmt.Sprintf("Failed to lookup %s %s", logKey, target), zap.Error(err))
		return err
	}

	// Successfully resolved with content. Save the result to attribute
	if len(result) > 0 {
		pMap.PutStr(config.ResolvedAttribute, result)
	}
	return nil
}

// targetStrFromAttributes returns the first IP/hostname from the given attributes.
// It uses the provided parsing function to check the format.
// If no valid IP/hostname is found, it returns an error.
func targetStrFromAttributes(attributes []string, pMap pcommon.Map, parseFn func(string) (string, error)) (string, error) {
	lastErr := errHostnameOrIPNotFound

	for _, attr := range attributes {
		if val, found := pMap.Get(attr); found {
			if parsedStr, err := parseFn(val.Str()); err == nil {
				return parsedStr, nil
			} else {
				lastErr = err
			}
		}
	}

	return "", lastErr
}
