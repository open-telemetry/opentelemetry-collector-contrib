// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dpfilters // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"

import (
	"errors"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
)

type dataPointFilter struct {
	metricFilter     *StringFilter
	dimensionsFilter *dimensionsFilter
}

// newDataPointFilter returns a new dataPointFilter filter with the given configuration.
func newDataPointFilter(metricNames []string, dimSet map[string][]string) (*dataPointFilter, error) {
	var metricFilter *StringFilter
	if len(metricNames) > 0 {
		var err error
		metricFilter, err = NewStringFilter(metricNames)
		if err != nil {
			return nil, err
		}
	}

	var dimensionsFilter *dimensionsFilter
	if len(dimSet) > 0 {
		var err error
		dimensionsFilter, err = newDimensionsFilter(dimSet)
		if err != nil {
			return nil, err
		}
	}

	if metricFilter == nil && dimensionsFilter == nil {
		return nil, errors.New("metric filter must have at least one metric or dimension defined on it")
	}

	return &dataPointFilter{
		metricFilter:     metricFilter,
		dimensionsFilter: dimensionsFilter,
	}, nil
}

// Matches tests a datapoint to see whether it is excluded by this
func (f *dataPointFilter) Matches(dp *sfxpb.DataPoint) bool {
	metricNameMatched := f.metricFilter == nil || f.metricFilter.Matches(dp.Metric)
	if metricNameMatched {
		return f.dimensionsFilter == nil || f.dimensionsFilter.Matches(dp.Dimensions)
	}
	return false

}
