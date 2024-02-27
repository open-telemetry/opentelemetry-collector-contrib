// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var _ resolver = (*dnssrvnoaResolver)(nil)

/*
TODO: These are from resolver_dns.go, but are used here. We should move them to a common place
const (
  defaultResInterval = 5 * time.Second
  defaultResTimeout  = time.Second
)
*/

var (
	errNoSRV       = errors.New("no SRV record found")
	errBadSRV      = errors.New("SRV hostname must be in the form of _service._proto.name")
	errNotSingleIP = errors.New("underlying A record must return a single IP address")

	dnssrvnoaResolverMutator = tag.Upsert(tag.MustNewKey("resolver"), "dnssrvnoa")

	dnssrvnoaResolverSuccessTrueMutators  = []tag.Mutator{dnssrvnoaResolverMutator, successTrueMutator}
	dnssrvnoaResolverSuccessFalseMutators = []tag.Mutator{dnssrvnoaResolverMutator, successFalseMutator}
)

type dnssrvnoaResolver struct {
	logger *zap.Logger

	srvService  string
	srvProto    string
	srvName     string
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

func newDNSSRVNOAResolver(logger *zap.Logger, srvHostname string, interval time.Duration, timeout time.Duration) (*dnssrvnoaResolver, error) {
	if len(srvHostname) == 0 {
		return nil, errNoSRV
	}
	if interval == 0 {
		interval = defaultResInterval
	}
	if timeout == 0 {
		timeout = defaultResTimeout
	}

	parsedSRVHostname, err := parseSRVHostname(srvHostname)
	if err != nil {
		logger.Warn("failed to parse SRV hostname")
		return nil, err
	}

	return &dnssrvnoaResolver{
		logger:      logger,
		srvService:  parsedSRVHostname.service,
		srvProto:    parsedSRVHostname.proto,
		srvName:     parsedSRVHostname.name,
		resolver:    &net.Resolver{},
		resInterval: interval,
		resTimeout:  timeout,
		stopCh:      make(chan struct{}),
	}, nil
}

func (r *dnssrvnoaResolver) start(ctx context.Context) error {
	if _, err := r.resolve(ctx); err != nil {
		r.logger.Warn("failed to resolve", zap.Error(err))
	}

	go r.periodicallyResolve()

	r.logger.Debug("SRV resolver started",
		zap.String("SRV name", r.srvName),
		zap.Duration("interval", r.resInterval),
		zap.Duration("timeout", r.resTimeout),
	)
	return nil
}

func (r *dnssrvnoaResolver) shutdown(_ context.Context) error {
	r.changeCallbackLock.Lock()
	r.onChangeCallbacks = nil
	r.changeCallbackLock.Unlock()

	close(r.stopCh)
	r.shutdownWg.Wait()
	return nil
}

func (r *dnssrvnoaResolver) periodicallyResolve() {
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

func (r *dnssrvnoaResolver) resolve(ctx context.Context) ([]string, error) {
	r.shutdownWg.Add(1)
	defer r.shutdownWg.Done()

	_, srvs, err := r.resolver.LookupSRV(ctx, r.srvService, r.srvProto, r.srvName)
	if err != nil {
		_ = stats.RecordWithTags(ctx, dnssrvnoaResolverSuccessFalseMutators, mNumResolutions.M(1))
		return nil, err
	}

	_ = stats.RecordWithTags(ctx, dnssrvnoaResolverSuccessTrueMutators, mNumResolutions.M(1))

	// backendsWithInfo stores the port and later the IP addresses for comparison
	backendsWithInfo := make(map[string]string)

	for _, srv := range srvs {
		target := strings.TrimSuffix(srv.Target, ".")
		port := strconv.FormatUint(uint64(srv.Port), 10)
		backendsWithInfo[target] = port
	}
	// backends is what we use to compare against the current endpoints
	var backends []string

	// freshBackends is used to compare against the existance of endpoints and if the IPs have changed
	var freshBackends []string

	// Lookup the IP addresses for the A records
	for aRec, port := range backendsWithInfo {

		// handle backends first
		backend := fmt.Sprintf("%s:%s", aRec, port)
		backends = append(backends, backend)

		ips, err := r.resolver.LookupIPAddr(ctx, aRec)
		// Return the A record. If we can't resolve them, we'll try again next iteration
		if err != nil {
			_ = stats.RecordWithTags(ctx, dnssrvnoaResolverSuccessFalseMutators, mNumResolutions.M(1))
			continue
		}
		// A headless Service SRV target only returns 1 IP address for its A record
		if len(ips) > 1 {
			return nil, errNotSingleIP
		}

		ip := ips[0]
		if ip.IP.To4() != nil {
			backendsWithInfo[aRec] = ip.String()
		} else {
			// it's an IPv6 address
			backendsWithInfo[aRec] = fmt.Sprintf("[%s]", ip.String())
		}

		// If the old map doesn't have the endpoint, it's fresh
		if _, ok := r.endpointsWithIPs[aRec]; !ok {
			freshBackends = append(freshBackends, backend)
			// If the old map has the endpoint and IPs match it's still fresh
			// Else freshBackends will be smaller and used later during callbacks
		} else if backendsWithInfo[aRec] == r.endpointsWithIPs[aRec] {
			freshBackends = append(freshBackends, backend)
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
	r.logger.Debug("Updating endpoints", zap.Strings("new endpoints", backends), zap.Any("old endpoints with IPs", r.endpointsWithIPs), zap.Any("new endpoints with IPs", backendsWithInfo))
	r.endpoints = backends
	r.endpointsWithIPs = backendsWithInfo
	r.updateLock.Unlock()
	_ = stats.RecordWithTags(ctx, dnssrvnoaResolverSuccessTrueMutators, mNumBackends.M(int64(len(backends))))

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

func (r *dnssrvnoaResolver) onChange(f func([]string)) {
	r.changeCallbackLock.Lock()
	defer r.changeCallbackLock.Unlock()
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}

type parsedSRVHostname struct {
	service string
	proto   string
	name    string
}

func parseSRVHostname(srvHostname string) (result *parsedSRVHostname, err error) {
	parts := strings.Split(srvHostname, ".")
	if len(parts) < 3 {
		return nil, errBadSRV
	}

	service := strings.TrimPrefix(parts[0], "_")
	proto := strings.TrimPrefix(parts[1], "_")
	name := strings.Join(parts[2:], ".")

	return &parsedSRVHostname{
		service: service,
		proto:   proto,
		name:    name,
	}, nil
}
