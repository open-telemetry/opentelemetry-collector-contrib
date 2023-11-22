// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

// TODO: What is this?
var _ resolver = (*srvResolver)(nil)

// TODO: Should these be moved to somethingl ike resolver_common.go?
// const (
// 	defaultResInterval = 5 * time.Second
// 	defaultResTimeout  = time.Second
// )

var (
	errNoSRV       = errors.New("no SRV record found")
	errBadSRV      = errors.New("SRV hostname must be in the form of _service._proto.name")
	errNotSingleIP = errors.New("underlying A record must return a single IP address")

	srvResolverMutator = tag.Upsert(tag.MustNewKey("resolver"), "srv")

	srvResolverSuccessTrueMutators  = []tag.Mutator{srvResolverMutator, successTrueMutator}
	srvResolverSuccessFalseMutators = []tag.Mutator{srvResolverMutator, successFalseMutator}
)

type srvResolver struct {
	logger *zap.Logger

	srvService  string
	srvProto    string
	srvName     string
	port        string
	resolver    multiResolver
	resInterval time.Duration
	resTimeout  time.Duration

	endpoints         []string
	endpointsWithIPs  map[string]string
	onChangeCallbacks []func([]string)

	stopCh             chan (struct{})
	updateLock         sync.Mutex
	shutdownWg         sync.WaitGroup
	changeCallbackLock sync.RWMutex
}

type multiResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
	LookupSRV(ctx context.Context, service, proto, name string) (cname string, addrs []*net.SRV, err error)
}

func newSRVResolver(logger *zap.Logger, srvHostname string, port string, interval time.Duration, timeout time.Duration) (*srvResolver, error) {
	if len(srvHostname) == 0 {
		return nil, errNoSRV
	}
	if interval == 0 {
		interval = defaultResInterval
	}
	if timeout == 0 {
		timeout = defaultResTimeout
	}

	service, proto, name, err := parseSRVHostname(srvHostname)
	if err != nil {
		logger.Warn("failed to parse SRV hostname", zap.Error(err))
	}

	return &srvResolver{
		logger:      logger,
		srvService:  service,
		srvProto:    proto,
		srvName:     name,
		port:        port,
		resolver:    &net.Resolver{},
		resInterval: interval,
		resTimeout:  timeout,
		stopCh:      make(chan struct{}),
	}, nil
}

func (r *srvResolver) start(ctx context.Context) error {
	if _, err := r.resolve(ctx); err != nil {
		r.logger.Warn("failed to resolve", zap.Error(err))
	}

	go r.periodicallyResolve()

	r.logger.Debug("SRV resolver started",
		zap.String("SRV name", r.srvName), zap.String("port", r.port),
		zap.Duration("interval", r.resInterval), zap.Duration("timeout", r.resTimeout))
	return nil
}

func (r *srvResolver) shutdown(_ context.Context) error {
	r.changeCallbackLock.Lock()
	r.onChangeCallbacks = nil
	r.changeCallbackLock.Unlock()

	close(r.stopCh)
	r.shutdownWg.Wait()
	return nil
}

func (r *srvResolver) periodicallyResolve() {
	ticker := time.NewTicker(r.resInterval)

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

func (r *srvResolver) resolve(ctx context.Context) ([]string, error) {
	r.shutdownWg.Add(1)
	defer r.shutdownWg.Done()

	_, srvs, err := r.resolver.LookupSRV(ctx, r.srvService, r.srvProto, r.srvName)
	if err != nil {
		_ = stats.RecordWithTags(ctx, srvResolverSuccessFalseMutators, mNumResolutions.M(1))
		return nil, err
	}

	_ = stats.RecordWithTags(ctx, srvResolverSuccessTrueMutators, mNumResolutions.M(1))

	// backendsWithIPs tracks the IP addresses for changes
	backendsWithIPs := make(map[string]string)
	for _, srv := range srvs {
		target := strings.TrimSuffix(srv.Target, ".")
		backendsWithIPs[target] = ""
	}
	// backends is what we use to compare against the current endpoints
	var backends []string

	// Lookup the IP addresses for the A records
	for aRec := range backendsWithIPs {

		// handle backends first
		backend := aRec
		// if a port is specified in the configuration, add it
		if r.port != "" {
			backend = fmt.Sprintf("%s:%s", backend, r.port)
		}
		backends = append(backends, backend)

		ips, err := r.resolver.LookupIPAddr(ctx, aRec)
		// Return the A record. If we can't resolve them, we'll try again next iteration
		if err != nil {
			_ = stats.RecordWithTags(ctx, srvResolverSuccessFalseMutators, mNumResolutions.M(1))
			continue
		}
		// A headless Service SRV target only returns 1 IP address for its A record
		if len(ips) > 1 {
			return nil, errNotSingleIP
		}

		ip := ips[0]
		if ip.IP.To4() != nil {
			backendsWithIPs[aRec] = ip.String()
		} else {
			// it's an IPv6 address
			backendsWithIPs[aRec] = fmt.Sprintf("[%s]", ip.String())
		}
	}

	var freshBackends []string
	for endpoint := range backendsWithIPs {
		// If the old map doesn't have the endpoint, it's fresh
		if _, ok := r.endpointsWithIPs[endpoint]; !ok {
			freshBackends = append(freshBackends, endpoint)
			// If the old map has the endpoint and IPs match it's still fresh
			// Else freshBackends will be smaller and used later during callbacks
		} else if backendsWithIPs[endpoint] == r.endpointsWithIPs[endpoint] {
			freshBackends = append(freshBackends, endpoint)
		}
	}

	// keep both in the same order
	slices.Sort(freshBackends)
	slices.Sort(backends)

	if equalStringSlice(r.endpoints, freshBackends) {
		r.logger.Debug("No change in endpoints")
		return r.endpoints, nil
	}

	// the list has changed!
	r.updateLock.Lock()
	r.logger.Debug("Updating endpoints", zap.Strings("new endpoints", backends))
	r.logger.Debug("Endpoints with IPs", zap.Any("old", r.endpointsWithIPs), zap.Any("new", backendsWithIPs))
	r.endpoints = backends
	r.endpointsWithIPs = backendsWithIPs
	r.updateLock.Unlock()
	_ = stats.RecordWithTags(ctx, srvResolverSuccessTrueMutators, mNumBackends.M(int64(len(backends))))

	// propagate the change
	r.changeCallbackLock.RLock()
	for _, callback := range r.onChangeCallbacks {
		// If backends != freshBackends it means an endpoint needs refreshed
		if !equalStringSlice(r.endpoints, freshBackends) {
			r.logger.Debug("Stale endpoints present", zap.Strings("fresh endpoints", freshBackends))
			callback(freshBackends)
		}
		callback(r.endpoints)
	}
	r.changeCallbackLock.RUnlock()

	return r.endpoints, nil
}

func (r *srvResolver) onChange(f func([]string)) {
	r.changeCallbackLock.Lock()
	defer r.changeCallbackLock.Unlock()
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}

func parseSRVHostname(srvHostname string) (service string, proto string, name string, err error) {
	parts := strings.Split(srvHostname, ".")
	if len(parts) < 3 {
		return "", "", "", errBadSRV
	}

	service = strings.TrimPrefix(parts[0], "_")
	proto = strings.TrimPrefix(parts[1], "_")
	name = strings.Join(parts[2:], ".")

	return service, proto, name, nil
}
