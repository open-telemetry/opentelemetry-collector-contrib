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
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/zap"
)

const (
	defaultPort = "4317"
)

var (
	errNoResolver                = errors.New("no resolvers specified for the exporter")
	errMultipleResolversProvided = errors.New("only one resolver should be specified")
)

var _ loadBalancer = (*loadBalancerImp)(nil)

type componentFactory func(ctx context.Context, endpoint string) (component.Component, error)

type loadBalancer interface {
	component.Component
	Endpoint(identifier []byte) string
	Exporter(endpoint string) (component.Component, error)
}

type loadBalancerImp struct {
	logger *zap.Logger
	host   component.Host

	res  resolver
	ring *hashRing

	componentFactory componentFactory
	exporters        map[string]component.Component

	stopped    bool
	updateLock sync.RWMutex
}

// Create new load balancer
func newLoadBalancer(params exporter.CreateSettings, cfg component.Config, factory componentFactory) (*loadBalancerImp, error) {
	oCfg := cfg.(*Config)

	if oCfg.Resolver.DNS != nil && oCfg.Resolver.Static != nil {
		return nil, errMultipleResolversProvided
	}

	var res resolver
	if oCfg.Resolver.Static != nil {
		var err error
		res, err = newStaticResolver(oCfg.Resolver.Static.Hostnames)
		if err != nil {
			return nil, err
		}
	}
	if oCfg.Resolver.DNS != nil {
		dnsLogger := params.Logger.With(zap.String("resolver", "dns"))

		var err error
		res, err = newDNSResolver(dnsLogger, oCfg.Resolver.DNS.Hostname, oCfg.Resolver.DNS.Port, oCfg.Resolver.DNS.Interval, oCfg.Resolver.DNS.Timeout)
		if err != nil {
			return nil, err
		}
	}

	if res == nil {
		return nil, errNoResolver
	}

	return &loadBalancerImp{
		logger:           params.Logger,
		res:              res,
		componentFactory: factory,
		exporters:        map[string]component.Component{},
	}, nil
}

func (lb *loadBalancerImp) Start(ctx context.Context, host component.Host) error {
	lb.res.onChange(lb.onBackendChanges)
	lb.host = host
	return lb.res.start(ctx)
}

func (lb *loadBalancerImp) onBackendChanges(resolved []string) {
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

func (lb *loadBalancerImp) addMissingExporters(ctx context.Context, endpoints []string) {
	for _, endpoint := range endpoints {
		endpoint = endpointWithPort(endpoint)

		if _, exists := lb.exporters[endpoint]; !exists {
			exp, err := lb.componentFactory(ctx, endpoint)
			if err != nil {
				lb.logger.Error("failed to create new exporter for endpoint", zap.String("endpoint", endpoint), zap.Error(err))
				continue
			}

			if err = exp.Start(ctx, lb.host); err != nil {
				lb.logger.Error("failed to start new exporter for endpoint", zap.String("endpoint", endpoint), zap.Error(err))
				continue
			}
			lb.exporters[endpoint] = exp
		}
	}
}

func endpointWithPort(endpoint string) string {
	if !strings.Contains(endpoint, ":") {
		endpoint = fmt.Sprintf("%s:%s", endpoint, defaultPort)
	}
	return endpoint
}

func (lb *loadBalancerImp) removeExtraExporters(ctx context.Context, endpoints []string) {
	endpointsWithPort := make([]string, len(endpoints))
	for i, e := range endpoints {
		endpointsWithPort[i] = endpointWithPort(e)
	}
	for existing := range lb.exporters {
		if !endpointFound(existing, endpointsWithPort) {
			_ = lb.exporters[existing].Shutdown(ctx)
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

func (lb *loadBalancerImp) Shutdown(context.Context) error {
	lb.stopped = true
	return nil
}

func (lb *loadBalancerImp) Endpoint(identifier []byte) string {
	lb.updateLock.RLock()
	defer lb.updateLock.RUnlock()

	return lb.ring.endpointFor(identifier)
}

func (lb *loadBalancerImp) Exporter(endpoint string) (component.Component, error) {
	// NOTE: make rolling updates of next tier of collectors work. currently, this may cause
	// data loss because the latest batches sent to outdated backend will never find their way out.
	// for details: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	lb.updateLock.RLock()
	exp, found := lb.exporters[endpointWithPort(endpoint)]
	lb.updateLock.RUnlock()
	if !found {
		// something is really wrong... how come we couldn't find the exporter??
		return nil, fmt.Errorf("couldn't find the exporter for the endpoint %q", endpoint)
	}

	return exp, nil
}
