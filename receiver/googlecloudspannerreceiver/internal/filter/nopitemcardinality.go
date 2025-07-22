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

func (*nopItemCardinalityFilter) Filter(sourceItems []*Item) []*Item {
	return sourceItems
}

func (*nopItemCardinalityFilter) Shutdown() error {
	return nil
}

func (*nopItemCardinalityFilter) TotalLimit() int {
	return 0
}

func (*nopItemCardinalityFilter) LimitByTimestamp() int {
	return 0
}

func (r *nopItemFilterResolver) Resolve(string) (ItemFilter, error) {
	return r.nopFilter, nil
}

func (*nopItemFilterResolver) Shutdown() error {
	return nil
}
