// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"

import "github.com/prometheus/prometheus/scrape"

// FakeMetadataStore implements scrape.MetricMetadataStore for tests.
// It is safe to use from other packages' tests
type FakeMetadataStore struct {
	data map[string]scrape.MetricMetadata
}

// NewFakeMetadataStore creates a FakeMetadataStore initialized with the given metadata.
func NewFakeMetadataStore(init map[string]scrape.MetricMetadata) *FakeMetadataStore {
	// copy defensively to avoid external mutation
	cp := make(map[string]scrape.MetricMetadata, len(init))
	for k, v := range init {
		cp[k] = v
	}
	return &FakeMetadataStore{data: cp}
}

func (f FakeMetadataStore) ListMetadata() []scrape.MetricMetadata {
	out := make([]scrape.MetricMetadata, 0, len(f.data))
	for _, m := range f.data {
		out = append(out, m)
	}
	return out
}

func (f FakeMetadataStore) GetMetadata(name string) (scrape.MetricMetadata, bool) {
	m, ok := f.data[name]
	return m, ok
}

func (f FakeMetadataStore) SizeMetadata() int {
	return len(f.data)
}

func (f FakeMetadataStore) LengthMetadata() int {
	return len(f.data)
}
