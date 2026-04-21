// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"sort"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

var _ resolver = (*staticResolver)(nil)

var (
	errNoEndpoints               = errors.New("no endpoints specified for the static resolver")
	staticResolverAttr           = attribute.String("resolver", "static")
	staticResolverAttrSet        = attribute.NewSet(staticResolverAttr)
	staticResolverSuccessAttrSet = attribute.NewSet(staticResolverAttr, attribute.Bool("success", true))
)

type staticResolver struct {
	endpoints         []string
	onChangeCallbacks []func([]string)
	once              sync.Once // we trigger the onChange only once

	telemetry *metadata.TelemetryBuilder
}

func newStaticResolver(endpoints []string, tb *metadata.TelemetryBuilder) (*staticResolver, error) {
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
		telemetry: tb,
	}, nil
}

func (r *staticResolver) start(ctx context.Context) error {
	_, err := r.resolve(ctx) // right now, this can't fail
	return err
}

func (r *staticResolver) shutdown(context.Context) error {
	r.endpoints = nil

	for _, callback := range r.onChangeCallbacks {
		callback(r.endpoints)
	}

	return nil
}

func (r *staticResolver) resolve(ctx context.Context) ([]string, error) {
	r.telemetry.LoadbalancerNumResolutions.Add(ctx, 1, metric.WithAttributeSet(staticResolverSuccessAttrSet))
	r.once.Do(func() {
		r.telemetry.LoadbalancerNumBackends.Record(ctx, int64(len(r.endpoints)), metric.WithAttributeSet(staticResolverAttrSet))
		r.telemetry.LoadbalancerNumBackendUpdates.Add(ctx, 1, metric.WithAttributeSet(staticResolverAttrSet))
		for _, callback := range r.onChangeCallbacks {
			callback(r.endpoints)
		}
	})
	return r.endpoints, nil
}

func (r *staticResolver) onChange(f func([]string)) {
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}
