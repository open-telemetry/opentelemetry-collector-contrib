// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dpfilters // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"

import (
	"errors"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
)

type dimensionsFilter struct {
	filterMap map[string]*StringFilter
}

// newDimensionsFilter returns a filter that matches against a
// sfxpb.Dimension slice. The filter will return false if there's
// at least one dimension in the slice that fails to match. In case`
// there are no filters for any of the dimension keys in the slice,
// the filter will return false.
func newDimensionsFilter(m map[string][]string) (*dimensionsFilter, error) {
	filterMap := map[string]*StringFilter{}
	for k := range m {
		if len(m[k]) == 0 {
			return nil, errors.New("string map value in filter cannot be empty")
		}

		var err error
		filterMap[k], err = NewStringFilter(m[k])
		if err != nil {
			return nil, err
		}
	}

	return &dimensionsFilter{
		filterMap: filterMap,
	}, nil
}

func (f *dimensionsFilter) Matches(dimensions []*sfxpb.Dimension) bool {
	if len(dimensions) == 0 {
		return false
	}

	var atLeastOneMatchedDimension bool
	for _, dim := range dimensions {
		dimF := f.filterMap[dim.Key]
		// Skip if there are no filters associated with current dimension key.
		if dimF == nil {
			continue
		}

		if !dimF.Matches(dim.Value) {
			return false
		}

		if !atLeastOneMatchedDimension {
			atLeastOneMatchedDimension = true
		}
	}

	return atLeastOneMatchedDimension
}
