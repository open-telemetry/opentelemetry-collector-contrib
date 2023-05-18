// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"

type nopItemCardinalityFilter struct {
	// No fields here
}

type nopItemFilterResolver struct {
	nopFilter *nopItemCardinalityFilter
}

func NewNopItemCardinalityFilter() ItemFilter {
	return &nopItemCardinalityFilter{}
}

func NewNopItemFilterResolver() ItemFilterResolver {
	return &nopItemFilterResolver{
		nopFilter: &nopItemCardinalityFilter{},
	}
}

func (f *nopItemCardinalityFilter) Filter(sourceItems []*Item) ([]*Item, error) {
	return sourceItems, nil
}

func (f *nopItemCardinalityFilter) Shutdown() error {
	return nil
}

func (f *nopItemCardinalityFilter) TotalLimit() int {
	return 0
}

func (f *nopItemCardinalityFilter) LimitByTimestamp() int {
	return 0
}

func (r *nopItemFilterResolver) Resolve(string) (ItemFilter, error) {
	return r.nopFilter, nil
}

func (r *nopItemFilterResolver) Shutdown() error {
	return nil
}
