// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package matcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"

import (
	"errors"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"
)

const (
	defaultOrderingCriteriaTopN = 1
)

var mtimeSortTypeFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"filelog.mtimeSortType",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, allows usage of `ordering_criteria.mode` = `mtime`."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27812"),
)

func New(c Criteria) (*Matcher, error) {
	m := &Matcher{
		include: c.Include,
		exclude: c.Exclude,
	}

	if c.ExcludeOlderThan != 0 {
		m.filterOpts = append(m.filterOpts, filter.ExcludeOlderThan(c.ExcludeOlderThan))
	}

	// Ignore empty string because that was the old behavior since `group_by` config was a string.
	if c.OrderingCriteria.GroupBy != nil && len(c.OrderingCriteria.GroupBy.String()) == 0 {
		m.groupBy = c.OrderingCriteria.GroupBy
	}

	if len(c.OrderingCriteria.SortBy) == 0 {
		return m, nil
	}

	if c.OrderingCriteria.TopN == 0 {
		c.OrderingCriteria.TopN = defaultOrderingCriteriaTopN
	}

	if orderingCriteriaNeedsRegex(c.OrderingCriteria.SortBy) {
		m.regex = c.OrderingCriteria.Regex
	}

	for _, sc := range c.OrderingCriteria.SortBy {
		switch sc.SortType {
		case sortTypeNumeric:
			m.filterOpts = append(m.filterOpts, filter.SortNumeric(sc.RegexKey, sc.Ascending))
		case sortTypeAlphabetical:
			m.filterOpts = append(m.filterOpts, filter.SortAlphabetical(sc.RegexKey, sc.Ascending))
		case sortTypeTimestamp:
			f, err := filter.SortTemporal(sc.RegexKey, sc.Ascending, sc.Layout, sc.Location)
			if err != nil {
				return nil, fmt.Errorf("timestamp sort: %w", err)
			}
			m.filterOpts = append(m.filterOpts, f)
		case sortTypeMtime:
			m.filterOpts = append(m.filterOpts, filter.SortMtime(sc.Ascending))
		}
	}

	m.filterOpts = append(m.filterOpts, filter.TopNOption(c.OrderingCriteria.TopN))

	return m, nil
}

// orderingCriteriaNeedsRegex returns true if any of the sort options require a regex to be set.
func orderingCriteriaNeedsRegex(sorts []Sort) bool {
	for _, s := range sorts {
		switch s.SortType {
		case sortTypeNumeric, sortTypeAlphabetical, sortTypeTimestamp:
			return true
		}
	}
	return false
}

type Matcher struct {
	include    []string
	exclude    []string
	regex      *regexp.Regexp
	filterOpts []filter.Option
	groupBy    *regexp.Regexp
}

// MatchFiles gets a list of paths given an array of glob patterns to include and exclude
func (m Matcher) MatchFiles() ([]string, error) {
	files, err := finder.FindFiles(m.include, m.exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}
	if len(files) == 0 {
		return nil, errors.New("no files match the configured criteria")
	}
	if len(m.filterOpts) == 0 {
		return files, nil
	}

	groups := make(map[string][]string)
	if m.groupBy != nil {
		for _, f := range files {
			matches := m.groupBy.FindStringSubmatch(f)
			if len(matches) > 1 {
				group := matches[1]
				groups[group] = append(groups[group], f)
			}
		}
	} else {
		groups["1"] = files
	}

	var result []string
	for _, groupedFiles := range groups {
		groupResult, err := filter.Filter(groupedFiles, m.regex, m.filterOpts...)
		if len(groupResult) == 0 {
			return groupResult, err
		}
		result = append(result, groupResult...)
	}

	return result, nil
}
