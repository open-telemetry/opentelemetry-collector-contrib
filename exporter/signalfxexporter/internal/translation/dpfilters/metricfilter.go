// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dpfilters // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"

import "fmt"

type MetricFilter struct {
	// A single metric name to match against.
	MetricName string `mapstructure:"metric_name"`
	// A list of metric names to match against.
	MetricNames []string `mapstructure:"metric_names"`
	// A map of dimension key/values to match against. All key/values must
	// match a datapoint for it to be matched. The map values can be either
	// a single string or a list of strings.
	Dimensions map[string]any `mapstructure:"dimensions"`
}

func (mf *MetricFilter) normalize() (map[string][]string, error) {
	if mf.MetricName != "" {
		mf.MetricNames = append(mf.MetricNames, mf.MetricName)
	}

	dimSet := map[string][]string{}
	for k, v := range mf.Dimensions {
		switch s := v.(type) {
		case []any:
			var newSet []string
			for _, iv := range s {
				newSet = append(newSet, fmt.Sprintf("%v", iv))
			}
			dimSet[k] = newSet
		case string:
			dimSet[k] = []string{s}
		default:
			return nil, fmt.Errorf("%v should be either a string or string list", v)
		}
	}

	return dimSet, nil
}
