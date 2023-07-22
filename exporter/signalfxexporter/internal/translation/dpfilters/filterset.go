// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dpfilters // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"

import sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"

// FilterSet is a collection of datapont filters, any one of which must match
// for a datapoint to be matched.
type FilterSet struct {
	excludeFilters []*dataPointFilter
	includeFilters []*dataPointFilter
}

// Matches sends a datapoint through each of the filters in the set and returns
// true if at least one of them matches the datapoint.
func (fs *FilterSet) Matches(dp *sfxpb.DataPoint) bool {
	for _, ex := range fs.excludeFilters {
		if ex.Matches(dp) {
			// If we match an exclusionary filter, run through each inclusion
			// filter and see if anything includes the metrics.
			for _, in := range fs.includeFilters {
				if in.Matches(dp) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func NewFilterSet(excludes []MetricFilter, includes []MetricFilter) (*FilterSet, error) {
	excludeSet, err := getDataPointFilters(excludes)
	if err != nil {
		return nil, err
	}

	includeSet, err := getDataPointFilters(includes)
	if err != nil {
		return nil, err
	}

	return &FilterSet{
		excludeFilters: excludeSet,
		includeFilters: includeSet,
	}, nil
}

func getDataPointFilters(metricFilters []MetricFilter) ([]*dataPointFilter, error) {
	var out []*dataPointFilter
	for _, f := range metricFilters {
		dimSet, err := f.normalize()
		if err != nil {
			return nil, err
		}

		dpf, err := newDataPointFilter(f.MetricNames, dimSet)
		if err != nil {
			return nil, err
		}

		out = append(out, dpf)
	}
	return out, nil
}
