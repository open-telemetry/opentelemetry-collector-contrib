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

var errNoEndpoints = errors.New("no endpoints specified for the static resolver")

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
	r.resolve(ctx) // right now, this can't fail
	return nil
}

func (r *staticResolver) shutdown(ctx context.Context) error {
	return nil
}

func (r *staticResolver) resolve(ctx context.Context) ([]string, error) {
	// the context to use for all metrics in this function
	ctx, _ = tag.New(ctx,
		tag.Upsert(tag.MustNewKey("resolver"), "static"),
		tag.Upsert(tag.MustNewKey("success"), "true"),
	)
	stats.Record(ctx, mNumResolutions.M(1))

	r.once.Do(func() {
		stats.Record(ctx, mNumBackends.M(int64(len(r.endpoints))))

		for _, callback := range r.onChangeCallbacks {
			callback(r.endpoints)
		}
	})
	return r.endpoints, nil
}

func (r *staticResolver) onChange(f func([]string)) {
	r.onChangeCallbacks = append(r.onChangeCallbacks, f)
}
