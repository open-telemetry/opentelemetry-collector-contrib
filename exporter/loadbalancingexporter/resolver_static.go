// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"sort"
	"sync"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

var _ resolver = (*staticResolver)(nil)

var (
	errNoEndpoints = errors.New("no endpoints specified for the static resolver")

	staticResolverMutators = []tag.Mutator{tag.Upsert(tag.MustNewKey("resolver"), "static"), successTrueMutator}
)

type staticResolver struct {
	endpoints         []string
	onChangeCallbacks []func([]string)
	once              sync.Once // we trigger the onChange only once
}

func newStaticResolver(endpoints []string) (*staticResolver, error) {
	if len(endpoints) == 0 {
		return nil, errNoEndpoints
	}

	// make sure we won't change the provided slice
	endpointsCopy := make([]string, len(endpoints))
	copy(endpointsCopy, endpoints)

	// sort is a guarantee that the order of endpoints doesn't matter
	sort.Strings(endpointsCopy)

	return &staticResolver{
		endpoints: endpointsCopy,
	}, nil
}

func (r *staticResolver) start(ctx context.Context) error {
	_, err := r.resolve(ctx) // right now, this can't fail
	return err
}

func (r *staticResolver) shutdown(ctx context.Context) error {
	return nil
}

func (r *staticResolver) resolve(ctx context.Context) ([]string, error) {
	_ = stats.RecordWithTags(ctx, staticResolverMutators, mNumResolutions.M(1))

	r.once.Do(func() {
		_ = stats.RecordWithTags(ctx, staticResolverMutators, mNumBackends.M(int64(len(r.endpoints))))

		for _, callback := range r.onChangeCallbacks {
			callback(r.endpoints)
		}
	})
	return r.endpoints, nil
}

func (r *staticResolver) onChange(f func([]string)) {
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}
