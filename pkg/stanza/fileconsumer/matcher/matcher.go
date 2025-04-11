// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package matcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"
)

const (
	sortTypeNumeric      = "numeric"
	sortTypeTimestamp    = "timestamp"
	sortTypeAlphabetical = "alphabetical"
	sortTypeMtime        = "mtime"
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

type Criteria struct {
	Include []string `mapstructure:"include,omitempty"`
	Exclude []string `mapstructure:"exclude,omitempty"`

	// ExcludeOlderThan allows excluding files whose modification time is older
	// than the specified age.
	ExcludeOlderThan time.Duration    `mapstructure:"exclude_older_than"`
	OrderingCriteria OrderingCriteria `mapstructure:"ordering_criteria,omitempty"`
}

type OrderingCriteria struct {
	Regex   string `mapstructure:"regex,omitempty"`
	TopN    int    `mapstructure:"top_n,omitempty"`
	SortBy  []Sort `mapstructure:"sort_by,omitempty"`
	GroupBy string `mapstructure:"group_by,omitempty"`
}

type Sort struct {
	SortType  string `mapstructure:"sort_type,omitempty"`
	RegexKey  string `mapstructure:"regex_key,omitempty"`
	Ascending bool   `mapstructure:"ascending,omitempty"`

	// Timestamp only
	Layout   string `mapstructure:"layout,omitempty"`
	Location string `mapstructure:"location,omitempty"`
}

func New(c Criteria) (*Matcher, error) {
	if len(c.Include) == 0 {
		return nil, errors.New("'include' must be specified")
	}

	if err := finder.Validate(c.Include); err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	if err := finder.Validate(c.Exclude); err != nil {
		return nil, fmt.Errorf("exclude: %w", err)
	}

	m := &Matcher{
		include: c.Include,
		exclude: c.Exclude,
	}

	if c.ExcludeOlderThan != 0 {
		m.filterOpts = append(m.filterOpts, filter.ExcludeOlderThan(c.ExcludeOlderThan))
	}

	if c.OrderingCriteria.GroupBy != "" {
		r, err := regexp.Compile(c.OrderingCriteria.GroupBy)
		if err != nil {
			return nil, fmt.Errorf("compile group_by regex: %w", err)
		}
		m.groupBy = r
	}

	if len(c.OrderingCriteria.SortBy) == 0 {
		return m, nil
	}

	if c.OrderingCriteria.TopN < 0 {
		return nil, errors.New("'top_n' must be a positive integer")
	}

	if c.OrderingCriteria.TopN == 0 {
		c.OrderingCriteria.TopN = defaultOrderingCriteriaTopN
	}

	if orderingCriteriaNeedsRegex(c.OrderingCriteria.SortBy) {
		if c.OrderingCriteria.Regex == "" {
			return nil, errors.New("'regex' must be specified when 'sort_by' is specified")
		}

		var err error
		regex, err := regexp.Compile(c.OrderingCriteria.Regex)
		if err != nil {
			return nil, fmt.Errorf("compile regex: %w", err)
		}

		m.regex = regex
	}

	for _, sc := range c.OrderingCriteria.SortBy {
		switch sc.SortType {
		case sortTypeNumeric:
			f, err := filter.SortNumeric(sc.RegexKey, sc.Ascending)
			if err != nil {
				return nil, fmt.Errorf("numeric sort: %w", err)
			}
			m.filterOpts = append(m.filterOpts, f)
		case sortTypeAlphabetical:
			f, err := filter.SortAlphabetical(sc.RegexKey, sc.Ascending)
			if err != nil {
				return nil, fmt.Errorf("alphabetical sort: %w", err)
			}
			m.filterOpts = append(m.filterOpts, f)
		case sortTypeTimestamp:
			f, err := filter.SortTemporal(sc.RegexKey, sc.Ascending, sc.Layout, sc.Location)
			if err != nil {
				return nil, fmt.Errorf("timestamp sort: %w", err)
			}
			m.filterOpts = append(m.filterOpts, f)
		case sortTypeMtime:
			if !mtimeSortTypeFeatureGate.IsEnabled() {
				return nil, fmt.Errorf("the %q feature gate must be enabled to use %q sort type", mtimeSortTypeFeatureGate.ID(), sortTypeMtime)
			}
			m.filterOpts = append(m.filterOpts, filter.SortMtime(sc.Ascending))
		default:
			return nil, errors.New("'sort_type' must be specified")
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
	var errs error
	files, err := finder.FindFiles(m.include, m.exclude)
	if err != nil {
		errs = errors.Join(errs, err)
	}
	if len(files) == 0 {
		return files, errors.Join(errors.New("no files match the configured criteria"), errs)
	}
	if len(m.filterOpts) == 0 {
		return files, errs
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
			return groupResult, errors.Join(err, errs)
		}
		result = append(result, groupResult...)
	}

	return result, errs
}
