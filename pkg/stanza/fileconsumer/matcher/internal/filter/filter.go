// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"

import (
	"fmt"
	"regexp"

	"go.uber.org/multierr"
)

type Option interface {
	// Returned error is for explanatory purposes only.
	// All options will be called regardless of error.
	apply([]*item) ([]*item, error)
}

func Filter(values []string, regex *regexp.Regexp, opts ...Option) ([]string, error) {
	var errs error
	items := make([]*item, 0, len(values))
	for _, value := range values {
		it, err := newItem(value, regex)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		items = append(items, it)
	}
	for _, opt := range opts {
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

type item struct {
	value    string
	captures map[string]string

	// Used when an Option is unable to interpret the value.
	// For example, a numeric sort may fail to parse the value as a number.
	err error
}

func newItem(value string, regex *regexp.Regexp) (*item, error) {
	if regex == nil {
		return &item{
			value: value,
		}, nil
	}

	match := regex.FindStringSubmatch(value)
	if match == nil {
		return nil, fmt.Errorf("'%s' does not match regex", value)
	}
	it := &item{
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
