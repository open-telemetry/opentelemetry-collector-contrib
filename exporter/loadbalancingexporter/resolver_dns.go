// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
)

var _ resolver = (*dnsResolver)(nil)

var errNoHostname = errors.New("no hostname specified to resolve the backends")

type dnsResolver struct {
	hostname string
	resolver netResolver

	endpoints         []string
	onChangeCallbacks []func([]string)

	updateLock sync.Mutex
}

type netResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

func newDNSResolver(hostname string) (*dnsResolver, error) {
	if len(hostname) == 0 {
		return nil, errNoHostname
	}

	return &dnsResolver{
		hostname: hostname,
		resolver: &net.Resolver{},
	}, nil
}

func (r *dnsResolver) start(ctx context.Context) error {
	if _, err := r.resolve(ctx); err != nil {
		return err
	}
	return nil
}

func (r *dnsResolver) shutdown(ctx context.Context) error {
	return nil
}

func (r *dnsResolver) resolve(ctx context.Context) ([]string, error) {
	addrs, err := r.resolver.LookupIPAddr(ctx, r.hostname)
	if err != nil {
		return nil, err
	}

	var backends []string
	for _, ip := range addrs {
		var backend string
		if ip.IP.To4() != nil {
			backend = ip.String()
		} else {
			// it's an IPv6 address
			backend = fmt.Sprintf("[%s]", ip.String())
		}
		backends = append(backends, backend)
	}

	// keep it always in the same order
	sort.Sort(sort.StringSlice(backends))

	if equalStringSlice(r.endpoints, backends) {
		return r.endpoints, nil
	}

	// the list has changed!
	r.updateLock.Lock()
	r.endpoints = backends
	r.updateLock.Unlock()

	// propate the change
	for _, callback := range r.onChangeCallbacks {
		callback(r.endpoints)
	}

	return r.endpoints, nil
}

func (r *dnsResolver) onChange(f func([]string)) {
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
