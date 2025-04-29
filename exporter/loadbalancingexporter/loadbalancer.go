// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
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

	componentFactory componentFactory
	exporters        map[string]*wrappedExporter

	stopped    bool
	updateLock sync.RWMutex
}

// Create new load balancer
func newLoadBalancer(logger *zap.Logger, cfg component.Config, factory componentFactory, telemetry *metadata.TelemetryBuilder) (*loadBalancer, error) {
	oCfg := cfg.(*Config)

	count := 0
	if oCfg.Resolver.DNS != nil {
		count++
	}
	if oCfg.Resolver.Static != nil {
		count++
	}
	if oCfg.Resolver.AWSCloudMap != nil {
		count++
	}
	if oCfg.Resolver.K8sSvc != nil {
		count++
	}
	if count > 1 {
		return nil, errMultipleResolversProvided
	}

	var res resolver
	if oCfg.Resolver.Static != nil {
		var err error
		res, err = newStaticResolver(
			oCfg.Resolver.Static.Hostnames,
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}
	if oCfg.Resolver.DNS != nil {
		dnsLogger := logger.With(zap.String("resolver", "dns"))

		var err error
		res, err = newDNSResolver(
			dnsLogger,
			oCfg.Resolver.DNS.Hostname,
			oCfg.Resolver.DNS.Port,
			oCfg.Resolver.DNS.Interval,
			oCfg.Resolver.DNS.Timeout,
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}
	if oCfg.Resolver.K8sSvc != nil {
		k8sLogger := logger.With(zap.String("resolver", "k8s service"))

		clt, err := newInClusterClient()
		if err != nil {
			return nil, err
		}
		res, err = newK8sResolver(
			clt,
			k8sLogger,
			oCfg.Resolver.K8sSvc.Service,
			oCfg.Resolver.K8sSvc.Ports,
			oCfg.Resolver.K8sSvc.Timeout,
			oCfg.Resolver.K8sSvc.ReturnHostnames,
			telemetry,
		)
		if err != nil {
			return nil, err
		}
	}

	if oCfg.Resolver.AWSCloudMap != nil {
		awsCloudMapLogger := logger.With(zap.String("resolver", "aws_cloud_map"))
		var err error
		res, err = newCloudMapResolver(
			awsCloudMapLogger,
			&oCfg.Resolver.AWSCloudMap.NamespaceName,
			&oCfg.Resolver.AWSCloudMap.ServiceName,
			oCfg.Resolver.AWSCloudMap.Port,
			&oCfg.Resolver.AWSCloudMap.HealthStatus,
			oCfg.Resolver.AWSCloudMap.Interval,
			oCfg.Resolver.AWSCloudMap.Timeout,
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
		if !endpointFound(existing, endpointsWithPort) {
			exp := lb.exporters[existing]
			// Shutdown the exporter asynchronously to avoid blocking the resolver
			go func() {
				_ = exp.Shutdown(ctx)
			}()
			delete(lb.exporters, existing)
		}
	}
}

func endpointFound(endpoint string, endpoints []string) bool {
	for _, candidate := range endpoints {
		if candidate == endpoint {
			return true
		}
	}

	return false
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
