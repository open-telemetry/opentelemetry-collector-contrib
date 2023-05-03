// Copyright The OpenTelemetry Authors
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

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"bytes"
	"errors"
	"regexp"
	"sort"
	"strings"

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
	// (Optional) List of label matchers that define matching rules to filter against
	// the labels of incoming metrics.
	LabelMatchers []*LabelMatcher `mapstructure:"label_matchers"`

	// metricRegexList is a list of compiled regexes for metric name selectors.
	metricRegexList []*regexp.Regexp
}

// LabelMatcher defines a label filtering rule against the labels of incoming metrics. Only metrics that
// match the rules will be used by the surrounding MetricDeclaration.
type LabelMatcher struct {
	// List of label names to filter by. Their corresponding values are concatenated using
	// the separator and matched against the specified regular expression.
	LabelNames []string `mapstructure:"label_names"`
	// (Optional) Separator placed between concatenated source label values. (Default: ';')
	Separator string `mapstructure:"separator"`
	// Regex string to be used to match against values of the concatenated labels.
	Regex string `mapstructure:"regex"`

	compiledRegex *regexp.Regexp
}

// dedupDimensionSet removes duplicated entries from dimension set.
func dedupDimensionSet(dimensions []string) (deduped []string, hasDuplicate bool) {
	seen := make(map[string]bool, len(dimensions))
	for _, v := range dimensions {
		seen[v] = true
	}
	hasDuplicate = (len(seen) < len(dimensions))
	if !hasDuplicate {
		deduped = dimensions
		return
	}
	deduped = make([]string, len(seen))
	idx := 0
	for dim := range seen {
		deduped[idx] = dim
		idx++
	}
	return
}

// init initializes the MetricDeclaration struct. Performs validation and compiles
// regex strings. Dimensions are deduped and sorted.
func (m *MetricDeclaration) init(logger *zap.Logger) (err error) {
	// Return error if no metric name selectors are defined
	if len(m.MetricNameSelectors) == 0 {
		return errors.New("invalid metric declaration: no metric name selectors defined")
	}

	// Filter out duplicate dimension sets and those with more than 10 elements
	validDims := make([][]string, 0, len(m.Dimensions))
	seen := make(map[string]bool, len(m.Dimensions))
	for _, dimSet := range m.Dimensions {
		concatenatedDims := strings.Join(dimSet, ",")
		if len(dimSet) > 10 {
			logger.Warn("Dropped dimension set: > 10 dimensions specified.", zap.String("dimensions", concatenatedDims))
			continue
		}

		// Dedup dimensions within dimension set
		dedupedDims, hasDuplicate := dedupDimensionSet(dimSet)
		if hasDuplicate {
			logger.Debug("Removed duplicates from dimension set.", zap.String("dimensions", concatenatedDims))
		}

		// Sort dimensions
		sort.Strings(dedupedDims)

		// Dedup dimension sets
		key := strings.Join(dedupedDims, ",")
		if _, ok := seen[key]; ok {
			logger.Debug("Dropped dimension set: duplicated dimension set.", zap.String("dimensions", concatenatedDims))
			continue
		}
		seen[key] = true
		validDims = append(validDims, dedupedDims)
	}
	m.Dimensions = validDims

	m.metricRegexList = make([]*regexp.Regexp, len(m.MetricNameSelectors))
	for i, selector := range m.MetricNameSelectors {
		m.metricRegexList[i] = regexp.MustCompile(selector)
	}

	// Initialize label matchers
	for _, lm := range m.LabelMatchers {
		if err := lm.init(); err != nil {
			return err
		}
	}
	return
}

// MatchesName returns true if the given OTLP Metric's name matches any of the Metric
// Declaration's metric name selectors.
func (m *MetricDeclaration) MatchesName(metricName string) bool {
	for _, regex := range m.metricRegexList {
		if regex.MatchString(metricName) {
			return true
		}
	}
	return false
}

// MatchesLabels returns true if the given OTLP Metric's name matches any of the Metric
// Declaration's label matchers.
func (m *MetricDeclaration) MatchesLabels(labels map[string]string) bool {
	if len(m.LabelMatchers) == 0 {
		return true
	}

	// If there are label matchers defined, check if metric's labels matches at least one
	for _, lm := range m.LabelMatchers {
		if lm.Matches(labels) {
			return true
		}
	}

	return false
}

// ExtractDimensions filters through the dimensions defined in the given metric declaration and
// returns dimensions that only contains labels from in the given label set.
func (m *MetricDeclaration) ExtractDimensions(labels map[string]string) (dimensions [][]string) {
	for _, dimensionSet := range m.Dimensions {
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

// init LabelMatcher with default values and compile regex string.
func (lm *LabelMatcher) init() (err error) {
	// Throw error if no label names are specified
	if len(lm.LabelNames) == 0 {
		return errors.New("label matcher must have at least one label name specified")
	}
	if len(lm.Regex) == 0 {
		return errors.New("regex not specified for label matcher")
	}
	if len(lm.Separator) == 0 {
		lm.Separator = ";"
	}

	lm.compiledRegex, err = regexp.Compile(lm.Regex)
	return err
}

// Matches returns true if given set of labels matches the LabelMatcher's rules.
func (lm *LabelMatcher) Matches(labels map[string]string) bool {
	concatenatedLabels := lm.getConcatenatedLabels(labels)
	return lm.compiledRegex.MatchString(concatenatedLabels)
}

// getConcatenatedLabels concatenates label values of matched labels using separator defined by the LabelMatcher's rules.
func (lm *LabelMatcher) getConcatenatedLabels(labels map[string]string) string {
	buf := new(bytes.Buffer)
	isFirstLabel := true
	for _, labelName := range lm.LabelNames {
		if isFirstLabel {
			isFirstLabel = false
		} else {
			buf.WriteString(lm.Separator)
		}

		buf.WriteString(labels[labelName])
	}
	return buf.String()
}
