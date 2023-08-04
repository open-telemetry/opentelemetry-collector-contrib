// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"

import (
	"regexp"

	"go.uber.org/multierr"
)

type Filter struct {
	items []*item
	opts  []Option
}

func (f *Filter) Apply() error {
	var errs error
	for _, opt := range f.opts {
		errs = multierr.Append(errs, opt.apply(f))
	}
	return errs
}

func (f *Filter) Values() []string {
	values := make([]string, 0, len(f.items))
	for _, item := range f.items {
		values = append(values, item.value)
	}
	return values
}

type Option interface {
	// Returned error is for explanitory purposes only.
	// All options will be called regardless of error.
	apply(*Filter) error
}

func New(values []string, regex *regexp.Regexp, opts ...Option) (Filter, error) {
	f := Filter{
		items: make([]*item, 0, len(values)),
		opts:  opts,
	}
	var errs error
	for _, value := range values {
		it, err := newItem(value, regex)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		f.items = append(f.items, it)
	}
	return f, errs
}
