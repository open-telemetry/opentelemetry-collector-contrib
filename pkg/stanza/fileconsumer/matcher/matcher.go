// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package matcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"
)

const (
	sortTypeNumeric      = "numeric"
	sortTypeTimestamp    = "timestamp"
	sortTypeAlphabetical = "alphabetical"
)

const (
	methodRegex = "regex"
	methodMtime = "mtime"
)

const (
	defaultOrderingCriteriaTopN = 1
)

type Criteria struct {
	Include          []string         `mapstructure:"include,omitempty"`
	Exclude          []string         `mapstructure:"exclude,omitempty"`
	OrderingCriteria OrderingCriteria `mapstructure:"ordering_criteria,omitempty"`
}

type OrderingCriteria struct {
	Method string `mapstructure:"method"`
	Regex  string `mapstructure:"regex,omitempty"`
	TopN   int    `mapstructure:"top_n,omitempty"`
	SortBy []Sort `mapstructure:"sort_by,omitempty"`
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
		return nil, fmt.Errorf("'include' must be specified")
	}

	if err := finder.Validate(c.Include); err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	if err := finder.Validate(c.Exclude); err != nil {
		return nil, fmt.Errorf("exclude: %w", err)
	}

	var f filter.Filter
	switch c.OrderingCriteria.Method {
	case methodMtime:
		f = filter.NewMTimeFilter()
	case methodRegex, "": // If no method is specified, defaults to regex
		// regex type with no SortBy indicates no-op
		if len(c.OrderingCriteria.SortBy) == 0 {
			return &Matcher{
				include: c.Include,
				exclude: c.Exclude,
			}, nil
		}

		if c.OrderingCriteria.Regex == "" {
			return nil, fmt.Errorf("'regex' must be specified when 'sort_by' is specified")
		}

		var err error
		f, err = createRegexFilter(c.OrderingCriteria)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid ordering_criteria.method %q", c.OrderingCriteria.Method)
	}

	if c.OrderingCriteria.TopN < 0 {
		return nil, fmt.Errorf("'top_n' must be a positive integer")
	}

	if c.OrderingCriteria.TopN == 0 {
		c.OrderingCriteria.TopN = defaultOrderingCriteriaTopN
	}

	return &Matcher{
		include: c.Include,
		exclude: c.Exclude,
		topN:    c.OrderingCriteria.TopN,
		f:       f,
	}, nil

}

func createRegexFilter(oc OrderingCriteria) (filter.Filter, error) {
	regex, err := regexp.Compile(oc.Regex)
	if err != nil {
		return nil, fmt.Errorf("compile regex: %w", err)
	}

	var filterOpts []filter.RegexFilterOption
	for _, sc := range oc.SortBy {
		switch sc.SortType {
		case sortTypeNumeric:
			f, err := filter.SortNumeric(sc.RegexKey, sc.Ascending)
			if err != nil {
				return nil, fmt.Errorf("numeric sort: %w", err)
			}
			filterOpts = append(filterOpts, f)
		case sortTypeAlphabetical:
			f, err := filter.SortAlphabetical(sc.RegexKey, sc.Ascending)
			if err != nil {
				return nil, fmt.Errorf("alphabetical sort: %w", err)
			}
			filterOpts = append(filterOpts, f)
		case sortTypeTimestamp:
			f, err := filter.SortTemporal(sc.RegexKey, sc.Ascending, sc.Layout, sc.Location)
			if err != nil {
				return nil, fmt.Errorf("timestamp sort: %w", err)
			}
			filterOpts = append(filterOpts, f)
		default:
			return nil, fmt.Errorf("'sort_type' must be specified")
		}
	}

	return filter.NewRegexFilter(regex, filterOpts...), nil
}

type Matcher struct {
	include []string
	exclude []string
	topN    int
	f       filter.Filter
}

// MatchFiles gets a list of paths given an array of glob patterns to include and exclude
func (m Matcher) MatchFiles() ([]string, error) {
	var errs error
	files, err := finder.FindFiles(m.include, m.exclude)
	if err != nil {
		errs = errors.Join(errs, err)
	}
	if len(files) == 0 {
		return files, errors.Join(fmt.Errorf("no files match the configured criteria"), errs)
	}

	if m.f == nil || m.f.SkipFiltering() {
		return files, errs
	}

	result, err := m.f.Filter(files)
	if len(result) == 0 {
		return result, errors.Join(err, errs)
	}

	if len(result) <= m.topN {
		return result, errors.Join(err, errs)
	}

	return result[:m.topN], errors.Join(err, errs)
}
