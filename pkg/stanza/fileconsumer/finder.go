// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"regexp"

	"github.com/bmatcuk/doublestar/v4"
	"go.uber.org/multierr"
)

type MatchingCriteria struct {
	Include          []string         `mapstructure:"include,omitempty"`
	Exclude          []string         `mapstructure:"exclude,omitempty"`
	OrderingCriteria OrderingCriteria `mapstructure:"ordering_criteria,omitempty"`
}

type OrderingCriteria struct {
	Regex  string         `mapstructure:"regex,omitempty"`
	SortBy []SortRuleImpl `mapstructure:"sort_by,omitempty"`
}

type NumericSortRule struct {
	BaseSortRule `mapstructure:",squash"`
}

type AlphabeticalSortRule struct {
	BaseSortRule `mapstructure:",squash"`
}

type TimestampSortRule struct {
	BaseSortRule `mapstructure:",squash"`
	Layout       string `mapstructure:"layout,omitempty"`
	Location     string `mapstructure:"location,omitempty"`
}

// Deprecated: [v0.82.0] This will be made internal in a future release, tentatively v0.83.0.
type BaseSortRule struct {
	RegexKey  string `mapstructure:"regex_key,omitempty"`
	Ascending bool   `mapstructure:"ascending,omitempty"`
	SortType  string `mapstructure:"sort_type,omitempty"`
}

// Deprecated: [v0.82.0] This will be made internal in a future release, tentatively v0.83.0.
type SortRuleImpl struct {
	sortRule
}

// Deprecated: [v0.82.0] Use MatchingCriteria instead. This will be removed in v0.83.0.
type Finder = MatchingCriteria

// FindFiles gets a list of paths given an array of glob patterns to include and exclude
//
// Deprecated: [v0.80.0] This will be made internal in a future release, tentatively v0.83.0.
func (f Finder) FindFiles() ([]string, error) {
	all := make([]string, 0, len(f.Include))
	for _, include := range f.Include {
		matches, _ := doublestar.FilepathGlob(include, doublestar.WithFilesOnly()) // compile error checked in build
	INCLUDE:
		for _, match := range matches {
			for _, exclude := range f.Exclude {
				if itMatches, _ := doublestar.PathMatch(exclude, match); itMatches {
					continue INCLUDE
				}
			}

			for _, existing := range all {
				if existing == match {
					continue INCLUDE
				}
			}

			all = append(all, match)
		}
	}

	return f.FindCurrent(all)
}

// FindCurrent gets the current file to read from a list of files if ordering_criteria is configured
// otherwise it returns the list of files.
//
// Deprecated: [v0.82.0] This will be made internal in a future release, tentatively v0.83.0.
func (f Finder) FindCurrent(files []string) ([]string, error) {
	if len(f.OrderingCriteria.SortBy) == 0 || files == nil || len(files) == 0 {
		return files, nil
	}

	re := regexp.MustCompile(f.OrderingCriteria.Regex)

	var errs error
	for _, SortPattern := range f.OrderingCriteria.SortBy {
		sortedFiles, err := SortPattern.sort(re, files)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		files = sortedFiles
	}

	return []string{files[0]}, errs
}
