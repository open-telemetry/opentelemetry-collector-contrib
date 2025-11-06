// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	defaultPort = "4317"
)

var (
	errNoResolver                = errors.New("no resolvers specified for the exporter")
	errMultipleResolversProvided = errors.New("only one resolver should be specified")
)

type componentFactory func(ctx context.Context, endpoint string) (component.Component, error)

type loadBalancer struct {
	logger *zap.Logger
	host   component.Host

	res  resolver
	ring *hashRing

	componentFactory componentFactory
	exporters        map[string]*wrappedExporter

	// Track unhealthy endpoints across all signal types
	unhealthyEndpoints map[string]time.Time
	healthLock         sync.RWMutex

	stopped    bool
	updateLock sync.RWMutex
}

// Create new load balancer
func newLoadBalancer(logger *zap.Logger, cfg component.Config, factory componentFactory, telemetry *metadata.TelemetryBuilder) (*loadBalancer, error) {
	oCfg := cfg.(*Config)

	count := 0
	if oCfg.Resolver.DNS.HasValue() {
		count++
	}
	if oCfg.Resolver.Static.HasValue() {
		count++
	}
	if oCfg.Resolver.AWSCloudMap.HasValue() {
		count++
	}
	if oCfg.Resolver.K8sSvc.HasValue() {
		count++
	}
	if count > 1 {
		return nil, errMultipleResolversProvided
	}

	var res resolver
	if oCfg.Resolver.Static.HasValue() {
		var err error
		res, err = newStaticResolver(
			oCfg.Resolver.Static.Get().Hostnames,
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}
	if oCfg.Resolver.DNS.HasValue() {
		dnsLogger := logger.With(zap.String("resolver", "dns"))

		var err error
		dnsResolver := oCfg.Resolver.DNS.Get()

		res, err = newDNSResolver(
			dnsLogger,
			dnsResolver.Hostname,
			dnsResolver.Port,
			dnsResolver.Interval,
			dnsResolver.Timeout,
			&dnsResolver.Quarantine,
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}
	if oCfg.Resolver.K8sSvc.HasValue() {
		k8sLogger := logger.With(zap.String("resolver", "k8s service"))

		clt, err := newInClusterClient()
		if err != nil {
			return nil, err
		}
		k8sSvcResolver := oCfg.Resolver.K8sSvc.Get()
		res, err = newK8sResolver(
			clt,
			k8sLogger,
			k8sSvcResolver.Service,
			k8sSvcResolver.Ports,
			k8sSvcResolver.Timeout,
			k8sSvcResolver.ReturnHostnames,
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}

	if oCfg.Resolver.AWSCloudMap.HasValue() {
		awsCloudMapLogger := logger.With(zap.String("resolver", "aws_cloud_map"))
		awsCloudMapResolver := oCfg.Resolver.AWSCloudMap.Get()
		var err error
		res, err = newCloudMapResolver(
			awsCloudMapLogger,
			&awsCloudMapResolver.NamespaceName,
			&awsCloudMapResolver.ServiceName,
			awsCloudMapResolver.Port,
			&awsCloudMapResolver.HealthStatus,
			awsCloudMapResolver.Interval,
			awsCloudMapResolver.Timeout,
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}

	if res == nil {
		return nil, errNoResolver
	}

	return &loadBalancer{
		logger:             logger,
		res:                res,
		componentFactory:   factory,
		exporters:          map[string]*wrappedExporter{},
		unhealthyEndpoints: make(map[string]time.Time),
	}, nil
}

func (lb *loadBalancer) Start(ctx context.Context, host component.Host) error {
	lb.res.onChange(lb.onBackendChanges)
	lb.host = host
	return lb.res.start(ctx)
}

func (lb *loadBalancer) onBackendChanges(resolved []string) {
	newRing := newHashRing(resolved)

	if !newRing.equal(lb.ring) {
		lb.updateLock.Lock()
		defer lb.updateLock.Unlock()

		lb.ring = newRing

		// TODO: set a timeout?
		ctx := context.Background()

		// add the missing exporters first
		lb.addMissingExporters(ctx, resolved)
		lb.removeExtraExporters(ctx, resolved)
	}
}

func (lb *loadBalancer) addMissingExporters(ctx context.Context, endpoints []string) {
	for _, endpoint := range endpoints {
		endpoint = endpointWithPort(endpoint)

		if _, exists := lb.exporters[endpoint]; !exists {
			exp, err := lb.componentFactory(ctx, endpoint)
			if err != nil {
				lb.logger.Error("failed to create new exporter for endpoint", zap.String("endpoint", endpoint), zap.Error(err))
				continue
			}
			we := newWrappedExporter(exp, endpoint)
			if err = we.Start(ctx, lb.host); err != nil {
				lb.logger.Error("failed to start new exporter for endpoint", zap.String("endpoint", endpoint), zap.Error(err))
				continue
			}
			lb.exporters[endpoint] = we
		}
	}
}

func endpointWithPort(endpoint string) string {
	if !strings.Contains(endpoint, ":") {
		endpoint = fmt.Sprintf("%s:%s", endpoint, defaultPort)
	}
	return endpoint
}

func (lb *loadBalancer) removeExtraExporters(ctx context.Context, endpoints []string) {
	endpointsWithPort := make([]string, len(endpoints))
	for i, e := range endpoints {
		endpointsWithPort[i] = endpointWithPort(e)
	}
	for existing := range lb.exporters {
		if !slices.Contains(endpointsWithPort, existing) {
			exp := lb.exporters[existing]
			// Shutdown the exporter asynchronously to avoid blocking the resolver
			go func() {
				_ = exp.Shutdown(ctx)
			}()
			delete(lb.exporters, existing)
		}
	}
}

// markUnhealthy marks an endpoint as unhealthy
func (lb *loadBalancer) markUnhealthy(endpoint string) {
	lb.healthLock.Lock()
	defer lb.healthLock.Unlock()

	if _, exists := lb.unhealthyEndpoints[endpoint]; !exists {
		lb.unhealthyEndpoints[endpoint] = time.Now()
	}
}

// isHealthy checks if an endpoint is healthy or if it has been quarantined long enough to retry
func (lb *loadBalancer) isHealthy(endpoint string) bool {
	lb.healthLock.RLock()
	timestamp, exists := lb.unhealthyEndpoints[endpoint]
	lb.healthLock.RUnlock()

	if !exists {
		return true
	}

	// If quarantine period has passed, remove from unhealthy list and allow retry
	if dnsRes, ok := lb.res.(*dnsResolver); ok && dnsRes.quarantine != nil {
		lb.logger.Debug("isHealthy", zap.String("endpoint", endpoint), zap.Time("timestamp", timestamp), zap.Duration("quarantineDuration", dnsRes.quarantine.Duration))
		if time.Since(timestamp) > dnsRes.quarantine.Duration {
			lb.healthLock.Lock()
			delete(lb.unhealthyEndpoints, endpoint)
			lb.healthLock.Unlock()
			lb.logger.Debug("isHealthy - quarantine period passed", zap.String("endpoint", endpoint))
			return true
		}
	}

	return false
}

// isQuarantineEnabled checks if the resolver supports quarantine logic and if it's enabled.
// Quarantine logic is supported for DNS resolvers only.
func (lb *loadBalancer) isQuarantineEnabled() bool {
	dnsRes, ok := lb.res.(*dnsResolver)
	return ok && dnsRes.quarantine.Enabled
}

func (lb *loadBalancer) Shutdown(ctx context.Context) error {
	err := lb.res.shutdown(ctx)
	lb.stopped = true

	for _, e := range lb.exporters {
		err = errors.Join(err, e.Shutdown(ctx))
	}
	return err
}

// exporterAndEndpoint returns the exporter and the endpoint for the given identifier.
func (lb *loadBalancer) exporterAndEndpoint(identifier []byte) (*wrappedExporter, string, error) {
	// NOTE: make rolling updates of next tier of collectors work. currently, this may cause
	// data loss because the latest batches sent to outdated backend will never find their way out.
	// for details: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	lb.updateLock.RLock()
	defer lb.updateLock.RUnlock()
	endpoint := lb.ring.endpointFor(identifier)
	exp, found := lb.exporters[endpointWithPort(endpoint)]
	if !found {
		// something is really wrong... how come we couldn't find the exporter??
		return nil, "", fmt.Errorf("couldn't find the exporter for the endpoint %q", endpoint)
	}

	return exp, endpoint, nil
}

// exporterAndEndpointByPosition returns the exporter and endpoint in the ring at the given position.
func (lb *loadBalancer) exporterAndEndpointByPosition(pos position) (*wrappedExporter, string, error) {
	lb.updateLock.RLock()
	defer lb.updateLock.RUnlock()

	endpoint := lb.ring.findEndpoint(pos)
	exp, found := lb.exporters[endpointWithPort(endpoint)]
	if !found {
		return nil, "", fmt.Errorf("couldn't find the exporter for the endpoint %q", endpoint)
	}

	return exp, endpoint, nil
}

// consumeWithRetryAndQuarantine executes the consume operation with the initial assignment of the exporter and endpoint.
// If the consume operation fails, it will retry with the next endpoint in its associated ring.
// It will try subsequent endpoints until either one succeeds or all available endpoints have been tried.
// Once an unhealthy endpoint is found, it will be marked as unhealthy and not tried again until the quarantine period has passed.
func (lb *loadBalancer) consumeWithRetryAndQuarantine(identifier []byte, exp *wrappedExporter, endpoint string, consume func(*wrappedExporter, string) error) error {
	var err error

	// Try the first endpoint if it's healthy or quarantine period has passed
	if lb.isHealthy(endpoint) {
		if err = consume(exp, endpoint); err == nil {
			return nil
		}
		// Mark as unhealthy if the consume failed
		lb.markUnhealthy(endpoint)
	}

	// If consume failed, try with subsequent endpoints
	// Keep track of tried endpoints to avoid infinite loop
	tried := map[string]bool{endpoint: true}
	currentPos := getPosition(identifier)

	// Try until we've used all available endpoints
	for len(tried) < len(lb.ring.endpoints) {
		// retryExp, retryEndpoint, retryErr := lb.exporterAndEndpoint(identifier)
		retryExp, retryEndpoint, retryErr := lb.exporterAndEndpointByPosition(currentPos)
		if retryErr != nil {
			// Return original error if we can't get a new endpoint
			return err
		}

		// If we've already tried this endpoint in this cycle, move to next position
		if tried[retryEndpoint] {
			currentPos = (currentPos + 1) % position(maxPositions)
			continue
		}

		// Skip unhealthy endpoints that are still in quarantine
		if !lb.isHealthy(retryEndpoint) {
			tried[retryEndpoint] = true
			// If we've exhausted all endpoints and they're all unhealthy, stop
			if len(tried) == len(lb.exporters) {
				break
			}
			currentPos = (currentPos + 1) % position(maxPositions)
			continue
		}

		tried[retryEndpoint] = true

		if retryErr = consume(retryExp, retryEndpoint); retryErr == nil {
			return nil
		}
		// Mark as unhealthy if the consume failed
		lb.markUnhealthy(retryEndpoint)

		// Move to next position for next iteration
		currentPos = (currentPos + 1) % position(maxPositions)
	}

	return fmt.Errorf("all endpoints were tried and failed: %v", tried)
}
