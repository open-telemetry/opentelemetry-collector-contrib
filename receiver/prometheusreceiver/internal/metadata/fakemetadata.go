// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import "github.com/prometheus/prometheus/scrape"

// FakeMetadataStore implements scrape.MetricMetadataStore for tests.
// It is safe to use from other packages' tests
type fakeMetadataStore struct {
	data map[string]scrape.MetricMetadata
}

// NewFakeMetadataStore creates a FakeMetadataStore initialized with the given metadata.
func NewFakeMetadataStore(init map[string]scrape.MetricMetadata) *fakeMetadataStore {
	// copy defensively to avoid external mutation
	cp := make(map[string]scrape.MetricMetadata, len(init))
	for k, v := range init {
		cp[k] = v
	}
	return &fakeMetadataStore{data: cp}
}

func (f fakeMetadataStore) ListMetadata() []scrape.MetricMetadata {
	out := make([]scrape.MetricMetadata, 0, len(f.data))
	for _, m := range f.data {
		out = append(out, m)
	}
	return out
}

func (f fakeMetadataStore) GetMetadata(name string) (scrape.MetricMetadata, bool) {
	m, ok := f.data[name]
	return m, ok
}

func (f fakeMetadataStore) SizeMetadata() int {
	return len(f.data)
}

func (f fakeMetadataStore) LengthMetadata() int {
	return len(f.data)
}
