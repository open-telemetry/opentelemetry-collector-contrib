// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

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

	componentFactory    componentFactory
	exporters           map[string]*wrappedExporter
	exportersShutdownWG sync.WaitGroup

	stopped    bool
	updateLock sync.RWMutex

	// backendChangeMu serializes the resolver callbacks, so that topology updates are still
	// applied one at a time without having to hold updateLock while exporters are being
	// created and started.
	backendChangeMu sync.Mutex
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
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}
	if oCfg.Resolver.K8sSvc.HasValue() {
		k8sLogger := logger.With(zap.String("resolver", "k8s service"))

		// Pass nil client - it will be created during start() to allow validation outside k8s cluster
		k8sSvcResolver := oCfg.Resolver.K8sSvc.Get()
		var err error
		res, err = newK8sResolver(
			nil,
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
			awsCloudMapResolver.OwnerAccount,
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
		logger:           logger,
		res:              res,
		componentFactory: factory,
		exporters:        map[string]*wrappedExporter{},
	}, nil
}

func (lb *loadBalancer) Start(ctx context.Context, host component.Host) error {
	lb.res.onChange(lb.onBackendChanges)
	lb.host = host
	return lb.res.start(ctx)
}

func (lb *loadBalancer) onBackendChanges(resolved []string) {
	lb.backendChangeMu.Lock()
	defer lb.backendChangeMu.Unlock()

	newRing := newHashRing(resolved)

	// prepare phase: take a snapshot of the current state, holding updateLock only for the
	// duration of the copy
	lb.updateLock.RLock()
	unchanged := newRing.equal(lb.ring)
	existing := maps.Clone(lb.exporters)
	lb.updateLock.RUnlock()

	if unchanged {
		return
	}

	// TODO: set a timeout?
	ctx := context.Background()

	// create and start the missing exporters without holding updateLock, so that slow
	// component factories or Start calls don't block the data path
	added := lb.startMissingExporters(ctx, resolved, existing)

	// commit phase: install the new exporters and publish the new ring atomically
	lb.updateLock.Lock()
	defer lb.updateLock.Unlock()

	maps.Copy(lb.exporters, added)
	lb.ring = newRing
	lb.removeExtraExporters(ctx, resolved)
}

// startMissingExporters creates and starts an exporter for every endpoint that isn't part of the
// given existing exporters, returning the successfully started ones keyed by endpoint. The returned
// exporters are not installed into the load balancer, so that partially constructed state stays
// private to the caller.
func (lb *loadBalancer) startMissingExporters(ctx context.Context, endpoints []string, existing map[string]*wrappedExporter) map[string]*wrappedExporter {
	added := make(map[string]*wrappedExporter)
	for _, endpoint := range endpoints {
		endpoint = endpointWithPort(endpoint)

		_, exists := existing[endpoint]
		if !exists {
			_, exists = added[endpoint]
		}

		if !exists {
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
			added[endpoint] = we
		}
	}

	return added
}

func (lb *loadBalancer) addMissingExporters(ctx context.Context, endpoints []string) {
	maps.Copy(lb.exporters, lb.startMissingExporters(ctx, endpoints, lb.exporters))
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
			lb.exportersShutdownWG.Go(func() {
				_ = exp.Shutdown(ctx)
			})
			delete(lb.exporters, existing)
		}
	}
}

func (lb *loadBalancer) Shutdown(ctx context.Context) error {
	err := lb.res.shutdown(ctx)
	lb.stopped = true

	for _, e := range lb.exporters {
		err = errors.Join(err, e.Shutdown(ctx))
	}
	lb.exportersShutdownWG.Wait()
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

// NumBackends returns the current number of resolved backend exporters.
func (lb *loadBalancer) NumBackends() int {
	lb.updateLock.RLock()
	defer lb.updateLock.RUnlock()
	return len(lb.exporters)
}
