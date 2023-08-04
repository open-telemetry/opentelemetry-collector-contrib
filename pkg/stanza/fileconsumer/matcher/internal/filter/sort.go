// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
)

type parseFunc func(string) (any, error)

type compareFunc func(a, b any) bool

type sortOption struct {
	regexKey string
	parseFunc
	compareFunc
}

func newSortOption(regexKey string, parseFunc parseFunc, compareFunc compareFunc) (Option, error) {
	if regexKey == "" {
		return nil, fmt.Errorf("regex key must be specified")
	}
	return sortOption{
		regexKey:    regexKey,
		parseFunc:   parseFunc,
		compareFunc: compareFunc,
	}, nil
}

func (o sortOption) apply(f *Filter) error {
	sort.Slice(f.items, func(i, j int) bool {
		// Parse both values before checking for errors
		valI, errI := o.parseFunc(f.items[i].captures[o.regexKey])
		valJ, errJ := o.parseFunc(f.items[j].captures[o.regexKey])
		if errI != nil && errJ != nil {
			f.items[i].err = errI
			f.items[j].err = errJ
			return true // Sort i to the top of the slice
		}
		if errI != nil {
			f.items[i].err = errI
			return true // Sort i to top of the slice
		}
		if errJ != nil {
			return false // Sort j to top of the slice
		}
		return o.compareFunc(valI, valJ)
	})

	// If there were errors, they are at the top of the slice.
	var errs error
	for i, it := range f.items {
		if it.err == nil {
			// No more errors, return the good items
			f.items = f.items[i:]
			return errs
		}
		errs = multierr.Append(errs, it.err)
	}

	// All items errored, clear the slice
	f.items = []*item{}
	return errs
}

func SortNumeric(regexKey string, ascending bool) (Option, error) {
	return newSortOption(regexKey,
		func(s string) (any, error) {
			return strconv.Atoi(s)
		},
		func(a, b any) bool {
			if ascending {
				return a.(int) < b.(int)
			}
			return a.(int) > b.(int)
		},
	)
}

func SortAlphabetical(regexKey string, ascending bool) (Option, error) {
	return newSortOption(regexKey,
		func(s string) (any, error) {
			return s, nil
		},
		func(a, b any) bool {
			if ascending {
				return a.(string) < b.(string)
			}
			return a.(string) > b.(string)
		},
	)
}

func SortTemporal(regexKey string, ascending bool, layout string, location string) (Option, error) {
	if layout == "" {
		return nil, fmt.Errorf("layout must be specified")
	}
	if location == "" {
		location = "UTC"
	}
	loc, err := timeutils.GetLocation(&location, &layout)
	if err != nil {
		return nil, fmt.Errorf("load location %s: %w", loc, err)
	}
	return newSortOption(regexKey,
		func(s string) (any, error) {
			return timeutils.ParseStrptime(layout, s, loc)
		},
		func(a, b any) bool {
			if ascending {
				return a.(time.Time).Before(b.(time.Time))
			}
			return a.(time.Time).After(b.(time.Time))
		},
	)
}
