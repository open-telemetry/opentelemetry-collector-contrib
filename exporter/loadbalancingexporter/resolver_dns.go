// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

var _ resolver = (*dnsResolver)(nil)

const (
	defaultResInterval = 5 * time.Second
	defaultResTimeout  = time.Second
)

var (
	errNoHostname = errors.New("no hostname specified to resolve the backends")

	aRecordResolverAttr           = attribute.String("resolver", "dns")
	aRecordResolverAttrSet        = attribute.NewSet(aRecordResolverAttr)
	aRecordResolverSuccessAttrSet = attribute.NewSet(aRecordResolverAttr, attribute.Bool("success", true))
	aRecordResolverFailureAttrSet = attribute.NewSet(aRecordResolverAttr, attribute.Bool("success", false))

	srvRecordResolverAttr           = attribute.String("resolver", "dnssrv")
	srvRecordResolverAttrSet        = attribute.NewSet(srvRecordResolverAttr)
	srvRecordResolverSuccessAttrSet = attribute.NewSet(srvRecordResolverAttr, attribute.Bool("success", true))
	srvRecordResolverFailureAttrSet = attribute.NewSet(srvRecordResolverAttr, attribute.Bool("success", false))
)

type dnsResolver struct {
	logger *zap.Logger

	hostname        string
	resolveBackends func(ctx context.Context) ([]string, error)
	resolverAttrSet attribute.Set
	resolver        netResolver
	resInterval     time.Duration
	resTimeout      time.Duration

	endpoints         []string
	onChangeCallbacks []func([]string)

	stopCh             chan struct{}
	updateLock         sync.Mutex
	shutdownWg         sync.WaitGroup
	changeCallbackLock sync.RWMutex
	telemetry          *metadata.TelemetryBuilder
}

type netResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
}

func newARecordDNSResolver(
	logger *zap.Logger,
	hostname string,
	port string,
	interval time.Duration,
	timeout time.Duration,
	tb *metadata.TelemetryBuilder,
) (*dnsResolver, error) {
	r, err := makeDNSResolver(logger, hostname, interval, timeout, tb)
	if err != nil {
		return nil, err
	}
	r.resolveBackends = func(ctx context.Context) ([]string, error) {
		return resolveARecord(ctx, r.resolver, hostname, port, tb)
	}
	r.resolverAttrSet = aRecordResolverAttrSet
	return r, nil
}

func newSRVRecordDNSResolver(
	logger *zap.Logger,
	hostname string,
	interval time.Duration,
	timeout time.Duration,
	tb *metadata.TelemetryBuilder,
) (*dnsResolver, error) {
	r, err := makeDNSResolver(logger, hostname, interval, timeout, tb)
	if err != nil {
		return nil, err
	}
	r.resolveBackends = func(ctx context.Context) ([]string, error) {
		return resolveSRV(ctx, r.resolver, hostname, logger, tb)
	}
	r.resolverAttrSet = srvRecordResolverAttrSet
	return r, nil
}

func makeDNSResolver(
	logger *zap.Logger,
	hostname string,
	interval time.Duration,
	timeout time.Duration,
	tb *metadata.TelemetryBuilder,
) (*dnsResolver, error) {
	if hostname == "" {
		return nil, errNoHostname
	}
	if interval == 0 {
		interval = defaultResInterval
	}
	if timeout == 0 {
		timeout = defaultResTimeout
	}

	return &dnsResolver{
		logger:      logger,
		hostname:    hostname,
		resolver:    &net.Resolver{},
		resInterval: interval,
		resTimeout:  timeout,
		stopCh:      make(chan struct{}),
		telemetry:   tb,
	}, nil
}

func (r *dnsResolver) start(ctx context.Context) error {
	if _, err := r.resolve(ctx); err != nil {
		r.logger.Warn("failed to resolve", zap.Error(err))
	}

	r.shutdownWg.Add(1)
	go r.periodicallyResolve()

	r.logger.Debug("DNS resolver started",
		zap.String("hostname", r.hostname),
		zap.Duration("interval", r.resInterval),
		zap.Duration("timeout", r.resTimeout),
	)
	return nil
}

func (r *dnsResolver) shutdown(_ context.Context) error {
	r.changeCallbackLock.Lock()
	r.onChangeCallbacks = nil
	r.changeCallbackLock.Unlock()

	close(r.stopCh)
	r.shutdownWg.Wait()
	return nil
}

func (r *dnsResolver) periodicallyResolve() {
	ticker := time.NewTicker(r.resInterval)
	defer ticker.Stop()
	defer r.shutdownWg.Done()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), r.resTimeout)
			if _, err := r.resolve(ctx); err != nil {
				r.logger.Warn("failed to resolve", zap.Error(err))
			} else {
				r.logger.Debug("resolved successfully")
			}
			cancel()
		case <-r.stopCh:
			return
		}
	}
}

func (r *dnsResolver) resolve(ctx context.Context) ([]string, error) {
	backends, err := r.resolveBackends(ctx)
	if err != nil {
		return nil, err
	}

	// keep it always in the same order
	sort.Strings(backends)

	if equalStringSlice(r.endpoints, backends) {
		return r.endpoints, nil
	}

	// the list has changed!
	r.updateLock.Lock()
	r.endpoints = backends
	r.updateLock.Unlock()

	r.telemetry.LoadbalancerNumBackends.Record(ctx, int64(len(backends)), metric.WithAttributeSet(r.resolverAttrSet))
	r.telemetry.LoadbalancerNumBackendUpdates.Add(ctx, 1, metric.WithAttributeSet(r.resolverAttrSet))

	// propagate the change
	r.changeCallbackLock.RLock()
	for _, callback := range r.onChangeCallbacks {
		callback(r.endpoints)
	}
	r.changeCallbackLock.RUnlock()

	return r.endpoints, nil
}

func resolveARecord(ctx context.Context, res netResolver, hostname, port string, tb *metadata.TelemetryBuilder) ([]string, error) {
	addrs, err := res.LookupIPAddr(ctx, hostname)
	if err != nil {
		tb.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(aRecordResolverFailureAttrSet))
		return nil, err
	}

	tb.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(aRecordResolverSuccessAttrSet))

	backends := make([]string, len(addrs))
	for i, ip := range addrs {
		var backend string
		if ip.IP.To4() != nil {
			backend = ip.String()
		} else {
			// it's an IPv6 address
			backend = fmt.Sprintf("[%s]", ip.String())
		}

		// if a port is specified in the configuration, add it
		if port != "" {
			backend = fmt.Sprintf("%s:%s", backend, port)
		}

		backends[i] = backend
	}

	return backends, nil
}

func resolveSRV(ctx context.Context, res netResolver, hostname string, logger *zap.Logger, tb *metadata.TelemetryBuilder) ([]string, error) {
	_, srvs, err := res.LookupSRV(ctx, "", "", hostname)
	if err != nil {
		tb.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(srvRecordResolverFailureAttrSet))
		return nil, err
	}

	tb.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(srvRecordResolverSuccessAttrSet))

	backends := []string{}

	for _, srv := range srvs {
		target := srv.Target
		ips, err := res.LookupIPAddr(ctx, target)
		if err != nil {
			logger.Warn("failed lookup of host for SRV target", zap.String("target", target), zap.Error(err))
			continue
		}
		for _, ip := range ips {
			var backend string
			if ip.IP.To4() != nil {
				backend = ip.String()
			} else {
				// it's an IPv6 address
				backend = fmt.Sprintf("[%s]", ip.String())
			}
			backend = fmt.Sprintf("%s:%d", backend, srv.Port)
			backends = append(backends, backend)
		}
	}

	return backends, nil
}

func (r *dnsResolver) onChange(f func([]string)) {
	r.changeCallbackLock.Lock()
	defer r.changeCallbackLock.Unlock()
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}

func equalStringSlice(source, candidate []string) bool {
	if len(source) != len(candidate) {
		return false
	}
	for i := range source {
		if source[i] != candidate[i] {
			return false
		}
	}

	return true
}
