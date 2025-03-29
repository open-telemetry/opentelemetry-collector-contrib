// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package matcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

type Criteria struct {
	FinderConfig `mapstructure:",squash"`

	// ExcludeOlderThan allows excluding files whose modification time is older
	// than the specified age.
	ExcludeOlderThan time.Duration    `mapstructure:"exclude_older_than"`
	OrderingCriteria OrderingCriteria `mapstructure:"ordering_criteria,omitempty"`
}

type FinderConfig struct {
	Include []string `mapstructure:"include,omitempty"`
	Exclude []string `mapstructure:"exclude,omitempty"`
}

func (cfg *FinderConfig) Validate() error {
	if len(cfg.Include) == 0 {
		return errors.New("'include' must be specified")
	}

	if err := validateGlobs(cfg.Include); err != nil {
		return fmt.Errorf("'include' is invalid: %w", err)
	}

	if err := validateGlobs(cfg.Exclude); err != nil {
		return fmt.Errorf("'exclude' is invalid: %w", err)
	}

	return nil
}

func validateGlobs(globs []string) error {
	for _, glob := range globs {
		_, err := doublestar.PathMatch(glob, "matchstring")
		if err != nil {
			return fmt.Errorf("parse glob: %w", err)
		}
	}
	return nil
}

type OrderingCriteria struct {
	Regex   *regexp.Regexp `mapstructure:"regex,omitempty"`
	TopN    int            `mapstructure:"top_n,omitempty"`
	SortBy  []Sort         `mapstructure:"sort_by,omitempty"`
	GroupBy *regexp.Regexp `mapstructure:"group_by,omitempty"`
}

func (cfg *OrderingCriteria) Validate() error {
	if len(cfg.SortBy) == 0 {
		return nil
	}

	if cfg.TopN < 0 {
		return fmt.Errorf("'top_n' must be a positive integer")
	}

	if orderingCriteriaNeedsRegex(cfg.SortBy) {
		// Ignore empty string because that was the old behavior since `regex` config was a string.
		if cfg.Regex == nil || len(cfg.Regex.String()) == 0 {
			return errors.New("'regex' must be specified when 'sort_by' is specified")
		}
	}

	return nil
}

type Sort struct {
	SortType  SortType `mapstructure:"sort_type,omitempty"`
	RegexKey  string   `mapstructure:"regex_key,omitempty"`
	Ascending bool     `mapstructure:"ascending,omitempty"`

	// Timestamp only
	Layout   string `mapstructure:"layout,omitempty"`
	Location string `mapstructure:"location,omitempty"`
}

func (cfg *Sort) Validate() error {
	if cfg.SortType == "" {
		return errors.New("'sort_type' must be specified")
	}

	switch cfg.SortType {
	case sortTypeNumeric, sortTypeAlphabetical, sortTypeTimestamp:
		return errors.New("`regex_key` must be specified")
	case sortTypeMtime:
		if !mtimeSortTypeFeatureGate.IsEnabled() {
			return fmt.Errorf("the %q feature gate must be enabled to use %q sort type", mtimeSortTypeFeatureGate.ID(), sortTypeMtime)
		}
	}

	return nil
}

type SortType string

var (
	sortTypeNumeric      = SortType("numeric")
	sortTypeTimestamp    = SortType("timestamp")
	sortTypeAlphabetical = SortType("alphabetical")
	sortTypeMtime        = SortType("mtime")
)

func (st *SortType) UnmarshalText(text []byte) error {
	switch strings.ToLower(string(text)) {
	case string(sortTypeNumeric):
		*st = sortTypeNumeric
		return nil
	case string(sortTypeAlphabetical):
		*st = sortTypeAlphabetical
		return nil
	case string(sortTypeTimestamp):
		*st = sortTypeTimestamp
		return nil
	case string(sortTypeMtime):
		*st = sortTypeMtime
		return nil
	}
	return errors.New("'sort_type' must be valid option")
}
