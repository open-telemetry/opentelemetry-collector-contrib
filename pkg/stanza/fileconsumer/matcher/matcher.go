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

	RefreshInterval time.Duration `mapstructure:"refresh_interval,omitempty"`
}

type OrderingCriteria struct {
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

	// mtime only
	MaxTime time.Duration `mapstructure:"max_time,omitempty"`
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

	if c.RefreshInterval.Seconds() == 0 {
		c.RefreshInterval = time.Minute
	}

	m := &Matcher{
		include:         c.Include,
		exclude:         c.Exclude,
		refreshInterval: c.RefreshInterval,
		cache:           newCache(),
	}

	if c.ExcludeOlderThan != 0 {
		m.filterOpts = append(m.filterOpts, filter.ExcludeOlderThan(c.ExcludeOlderThan))
	}

	if len(c.OrderingCriteria.SortBy) == 0 {
		return m, nil
	}

	if c.OrderingCriteria.TopN < 0 {
		return nil, fmt.Errorf("'top_n' must be a positive integer")
	}

	if c.OrderingCriteria.TopN == 0 {
		c.OrderingCriteria.TopN = defaultOrderingCriteriaTopN
	}
	m.topN = c.OrderingCriteria.TopN

	var regex *regexp.Regexp
	if orderingCriteriaNeedsRegex(c.OrderingCriteria.SortBy) {
		if c.OrderingCriteria.Regex == "" {
			return nil, fmt.Errorf("'regex' must be specified when 'sort_by' is specified")
		}

		var err error
		regex, err = regexp.Compile(c.OrderingCriteria.Regex)
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
			m.maxAge = sc.MaxTime
			m.filterOpts = append(m.filterOpts, filter.SortMtime(sc.MaxTime))
		default:
			return nil, fmt.Errorf("'sort_type' must be specified")
		}
	}

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

// cache stores the matched files and last updated time. No mutex is used since all calls are sequential
type cache struct {
	lastUpdatedTime time.Time

	files []string
}

func newCache() *cache {
	return &cache{}
}

func (c *cache) getFiles() []string {
	return c.files
}

func (c *cache) update(files []string) {
	c.files = files
	c.lastUpdatedTime = time.Now()
}

func (c *cache) getLastUpdatedTime() time.Time {
	return c.lastUpdatedTime
}

type Matcher struct {
	include    []string
	exclude    []string
	regex      *regexp.Regexp
	topN       int
	filterOpts []filter.Option

	refreshInterval time.Duration
	maxAge          time.Duration
	cache           *cache
}

// MatchFiles gets a list of paths given an array of glob patterns to include and exclude
func (m Matcher) MatchFiles() ([]string, error) {
	var err, errs error

	files := m.cache.getFiles()
	if time.Since(m.cache.getLastUpdatedTime()) < m.refreshInterval {
		return files, nil
	}

	files, err = finder.FindFiles(m.include, m.exclude, m.maxAge)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	if len(files) == 0 {
		return files, errors.Join(fmt.Errorf("no files match the configured criteria"), errs)
	}
	if len(m.filterOpts) == 0 {
		return files, errs
	}

	files, err = filter.Filter(files, m.regex, m.filterOpts...)
	if len(files) == 0 {
		return files, errors.Join(err, errs)
	}

	if len(files) <= m.topN {
		m.cache.update(files)
		return files, errors.Join(err, errs)
	}

	files = files[:m.topN]

	m.cache.update(files)

	return files, errors.Join(err, errs)
}
