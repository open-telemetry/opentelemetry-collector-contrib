// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"go.uber.org/multierr"
)

type RegexFilterOption interface {
	// Returned error is for explanatory purposes only.
	// All options will be called regardless of error.
	apply([]*regexItem) ([]*regexItem, error)
}

type Filter interface {
	SkipFiltering() bool
	Filter(values []string) ([]string, error)
}

type RegexFilter struct {
	regex *regexp.Regexp
	opts  []RegexFilterOption
}

func NewRegexFilter(regex *regexp.Regexp, opts ...RegexFilterOption) RegexFilter {
	return RegexFilter{
		regex: regex,
		opts:  opts,
	}
}

func (r RegexFilter) SkipFiltering() bool {
	return len(r.opts) == 0
}

func (r RegexFilter) Filter(values []string) ([]string, error) {
	var errs error
	items := make([]*regexItem, 0, len(values))
	for _, value := range values {
		it, err := newRegexItem(value, r.regex)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		items = append(items, it)
	}
	for _, opt := range r.opts {
		var applyErr error
		items, applyErr = opt.apply(items)
		errs = multierr.Append(errs, applyErr)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.value)
	}
	return result, errs
}

type regexItem struct {
	value    string
	captures map[string]string

	// Used when an Option is unable to interpret the value.
	// For example, a numeric sort may fail to parse the value as a number.
	err error
}

func newRegexItem(value string, regex *regexp.Regexp) (*regexItem, error) {
	match := regex.FindStringSubmatch(value)
	if match == nil {
		return nil, fmt.Errorf("'%s' does not match regex", value)
	}
	it := &regexItem{
		value:    value,
		captures: make(map[string]string),
	}
	for i, name := range regex.SubexpNames() {
		if i == 0 || name == "" {
			continue
		}
		it.captures[name] = match[i]
	}
	return it, nil
}

type MTimeFilter struct{}

func NewMTimeFilter() MTimeFilter {
	return MTimeFilter{}
}

func (m MTimeFilter) SkipFiltering() bool {
	return false
}

type mTimeItem struct {
	mtime time.Time
	path  string
}

func (m MTimeFilter) Filter(values []string) ([]string, error) {
	items := make([]mTimeItem, 0, len(values))
	var errs error
	for _, filePath := range values {
		fi, err := os.Stat(filePath)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		items = append(items, mTimeItem{
			mtime: fi.ModTime(),
			path:  filePath,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		// This checks if item i > j, in order to reverse the sort (most recently modified file is first in the list)
		return items[i].mtime.After(items[j].mtime)
	})

	filteredValues := make([]string, 0, len(items))
	for _, item := range items {
		filteredValues = append(filteredValues, item.path)
	}

	return filteredValues, nil
}
