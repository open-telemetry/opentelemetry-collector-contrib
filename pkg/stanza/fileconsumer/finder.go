// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"regexp"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/finder"
)

type MatchingCriteria struct {
	Include          []string         `mapstructure:"include,omitempty"`
	Exclude          []string         `mapstructure:"exclude,omitempty"`
	OrderingCriteria OrderingCriteria `mapstructure:"ordering_criteria,omitempty"`
}

type OrderingCriteria struct {
	Regex  string         `mapstructure:"regex,omitempty"`
	SortBy []sortRuleImpl `mapstructure:"sort_by,omitempty"`
}

type NumericSortRule struct {
	baseSortRule `mapstructure:",squash"`
}

type AlphabeticalSortRule struct {
	baseSortRule `mapstructure:",squash"`
}

type TimestampSortRule struct {
	baseSortRule `mapstructure:",squash"`
	Layout       string `mapstructure:"layout,omitempty"`
	Location     string `mapstructure:"location,omitempty"`
}

type baseSortRule struct {
	RegexKey  string `mapstructure:"regex_key,omitempty"`
	Ascending bool   `mapstructure:"ascending,omitempty"`
	SortType  string `mapstructure:"sort_type,omitempty"`
}

type sortRuleImpl struct {
	sortRule
}

// findFiles gets a list of paths given an array of glob patterns to include and exclude
func (f MatchingCriteria) findFiles() ([]string, error) {
	all := finder.FindFiles(f.Include, f.Exclude)

	if len(all) == 0 || len(f.OrderingCriteria.SortBy) == 0 {
		return all, nil
	}

	re := regexp.MustCompile(f.OrderingCriteria.Regex)

	var errs error
	for _, SortPattern := range f.OrderingCriteria.SortBy {
		sortedFiles, err := SortPattern.sort(re, all)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		all = sortedFiles
	}

	return []string{all[0]}, errs
}
