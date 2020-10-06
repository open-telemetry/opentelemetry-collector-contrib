// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"errors"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// MetricDeclaration characterizes a rule to be used to set dimensions for certain
// incoming metrics, filtered by their metric names.
type MetricDeclaration struct {
	// Dimensions is a list of dimension sets (which are lists of dimension names) to be
	// included in exported metrics. If the metric does not contain any of the specified
	// dimensions, the metric would be dropped (will only show up in logs).
	Dimensions [][]string `mapstructure:"dimensions"`
	// MetricNameSelectors is a list of regex strings to be matched against metric names
	// to determine which metrics should be included with this metric declaration rule.
	MetricNameSelectors []string `mapstructure:"metric_name_selectors"`

	// metricRegexList is a list of compiled regexes for metric name selectors.
	metricRegexList []*regexp.Regexp
}

// Init initializes the MetricDeclaration struct. Performs validation and compiles
// regex strings.
func (m *MetricDeclaration) Init(logger *zap.Logger) (err error) {
	// Return error if no metric name selectors are defined
	if len(m.MetricNameSelectors) == 0 {
		return errors.New("Invalid metric declaration: no metric name selectors defined.")
	}

	// Filter out dimension sets with more than 10 elements
	validDims := make([][]string, 0, len(m.Dimensions))
	for _, dimSet := range m.Dimensions {
		if len(dimSet) <= 10 {
			validDims = append(validDims, dimSet)
		} else {
			logger.Warn("Dropped dimension set: > 10 dimensions specified.", zap.String("dimensions", strings.Join(dimSet, ",")))
		}
	}
	m.Dimensions = validDims

	m.metricRegexList = make([]*regexp.Regexp, len(m.MetricNameSelectors))
	for i, selector := range m.MetricNameSelectors {
		m.metricRegexList[i] = regexp.MustCompile(selector)
	}
	return
}

// Matches returns true if the given OTLP Metric's name matches any of the Metric
// Declaration's metric name selectors.
func (m *MetricDeclaration) Matches(metric *pdata.Metric) bool {
	for _, regex := range m.metricRegexList {
		if regex.MatchString(metric.Name()) {
			return true
		}
	}
	return false
}

// ExtractDimensions extracts dimensions within the MetricDeclaration that only
// contains labels found in `labels`.
func (m *MetricDeclaration) ExtractDimensions(labels map[string]string) (dimensions [][]string) {
	for _, dimensionSet := range m.Dimensions {
		if len(dimensionSet) == 0 {
			continue
		}
		includeSet := true
		for _, dim := range dimensionSet {
			if _, ok := labels[dim]; !ok {
				includeSet = false
				break
			}
		}
		if includeSet {
			dimensions = append(dimensions, dimensionSet)
		}
	}
	return
}

// processMetricDeclarations processes a list of MetricDeclarations and returns a
// list of dimensions that matches the given `metric`.
func processMetricDeclarations(metricDeclarations []*MetricDeclaration, metric *pdata.Metric, labels map[string]string) (dimensionsList [][][]string) {
	for _, m := range metricDeclarations {
		if m.Matches(metric) {
			dimensions := m.ExtractDimensions(labels)
			if len(dimensions) > 0 {
				dimensionsList = append(dimensionsList, dimensions)
			}
		}
	}
	return
}
