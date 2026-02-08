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

// DNSResolveMode defines the DNS resolution mode
type DNSResolveMode string

const (
	// DNSModeStandard uses standard A/AAAA record resolution
	DNSModeStandard DNSResolveMode = "standard"
	// DNSModeSRV uses SRV record resolution followed by A/AAAA resolution for each target
	DNSModeSRV DNSResolveMode = "srv"
)

var (
	errNoHostname = errors.New("no hostname specified to resolve the backends")

	standardResolverAttr           = attribute.String("resolver", "dns")
	standardResolverAttrSet        = attribute.NewSet(standardResolverAttr)
	standardResolverSuccessAttrSet = attribute.NewSet(standardResolverAttr, attribute.Bool("success", true))
	standardResolverFailureAttrSet = attribute.NewSet(standardResolverAttr, attribute.Bool("success", false))

	srvResolverAttr           = attribute.String("resolver", "dnssrv")
	srvResolverAttrSet        = attribute.NewSet(srvResolverAttr)
	srvResolverSuccessAttrSet = attribute.NewSet(srvResolverAttr, attribute.Bool("success", true))
	srvResolverFailureAttrSet = attribute.NewSet(srvResolverAttr, attribute.Bool("success", false))
)

type dnsResolver struct {
	logger *zap.Logger

	hostname    string
	port        string // Only used in standard mode
	mode        DNSResolveMode
	resolver    netResolver
	resInterval time.Duration
	resTimeout  time.Duration

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

func newDNSResolver(
	logger *zap.Logger,
	hostname string,
	port string,
	mode DNSResolveMode,
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
	if mode == "" {
		mode = DNSModeStandard
	}

	return &dnsResolver{
		logger:      logger,
		hostname:    hostname,
		port:        port,
		mode:        mode,
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

	logFields := []zap.Field{
		zap.String("hostname", r.hostname),
		zap.String("mode", string(r.mode)),
		zap.Duration("interval", r.resInterval),
		zap.Duration("timeout", r.resTimeout),
	}
	if r.mode == DNSModeStandard && r.port != "" {
		logFields = append(logFields, zap.String("port", r.port))
	}
	r.logger.Debug("DNS resolver started", logFields...)
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
	var backends []string
	var err error

	switch r.mode {
	case DNSModeSRV:
		backends, err = r.resolveSRV(ctx)
	case DNSModeStandard:
	default:
		backends, err = r.resolveStandard(ctx)
	}

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

	// Record metrics based on mode
	attrSet := r.getAttributeSet()
	r.telemetry.LoadbalancerNumBackends.Record(ctx, int64(len(backends)), metric.WithAttributeSet(attrSet))
	r.telemetry.LoadbalancerNumBackendUpdates.Add(ctx, 1, metric.WithAttributeSet(attrSet))

	// propagate the change
	r.changeCallbackLock.RLock()
	for _, callback := range r.onChangeCallbacks {
		callback(r.endpoints)
	}
	r.changeCallbackLock.RUnlock()

	return r.endpoints, nil
}

func (r *dnsResolver) resolveStandard(ctx context.Context) ([]string, error) {
	addrs, err := r.resolver.LookupIPAddr(ctx, r.hostname)
	if err != nil {
		r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(standardResolverFailureAttrSet))
		return nil, err
	}

	r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(standardResolverSuccessAttrSet))

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
		if r.port != "" {
			backend = fmt.Sprintf("%s:%s", backend, r.port)
		}

		backends[i] = backend
	}

	return backends, nil
}

func (r *dnsResolver) resolveSRV(ctx context.Context) ([]string, error) {
	_, srvs, err := r.resolver.LookupSRV(ctx, "", "", r.hostname)
	if err != nil {
		r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(srvResolverFailureAttrSet))
		return nil, err
	}

	r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(srvResolverSuccessAttrSet))

	backends := []string{}

	// DNS SRV mode: resolve A/AAAA for each SRV target
	for _, srv := range srvs {
		target := srv.Target
		ips, err := r.resolver.LookupIPAddr(ctx, target)
		if err != nil {
			r.logger.Warn("failed lookup of host for SRV target", zap.String("target", target), zap.Error(err))
			continue
		}
		for _, ip := range ips {
			var backend string
			if ip.IP.To4() != nil {
				backend = ip.String()
			} else {
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

func (r *dnsResolver) getAttributeSet() attribute.Set {
	if r.mode == DNSModeSRV {
		return srvResolverAttrSet
	}
	return standardResolverAttrSet
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
