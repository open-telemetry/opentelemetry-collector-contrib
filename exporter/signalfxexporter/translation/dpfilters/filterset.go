// Copyright 2021, OpenTelemetry Authors
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

package dpfilters

import sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"

// FilterSet is a collection of datapont filters, any one of which must match
// for a datapoint to be matched.
type FilterSet struct {
	ExcludeFilters []*dataPointFilter
}

// Matches sends a datapoint through each of the filters in the set and returns
// true if at least one of them matches the datapoint.
func (fs *FilterSet) Matches(dp *sfxpb.DataPoint) bool {
	for _, ex := range fs.ExcludeFilters {
		if ex.Matches(dp) {
			return true
		}
	}
	return false
}

func NewFilterSet(excludes []MetricFilter) (*FilterSet, error) {
	var excludeSet []*dataPointFilter
	for _, f := range excludes {
		dimSet, err := f.normalize()
		if err != nil {
			return nil, err
		}

		dpf, err := newDataPointFilter(f.MetricNames, dimSet)
		if err != nil {
			return nil, err
		}

		excludeSet = append(excludeSet, dpf)
	}
	return &FilterSet{
		ExcludeFilters: excludeSet,
	}, nil
}
