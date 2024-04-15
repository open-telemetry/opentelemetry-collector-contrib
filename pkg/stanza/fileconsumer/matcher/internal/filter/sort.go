// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
)

type parseFunc func(string) (any, error)

type compareFunc func(a, b any) bool

type regexSortOption struct {
	regexKey string
	parseFunc
	compareFunc
}

func newRegexSortOption(regexKey string, parseFunc parseFunc, compareFunc compareFunc) (Option, error) {
	if regexKey == "" {
		return nil, fmt.Errorf("regex key must be specified")
	}
	return regexSortOption{
		regexKey:    regexKey,
		parseFunc:   parseFunc,
		compareFunc: compareFunc,
	}, nil
}

func (o regexSortOption) apply(items []*item) ([]*item, error) {
	// Special case where sort.Slice will not run the 'less' func.
	// We still need to ensure it parses in order to ensure the file should be included.
	if len(items) == 1 {
		_, err := o.parseFunc(items[0].captures[o.regexKey])
		if err != nil {
			return []*item{}, err
		}
		return items, nil
	}

	sort.Slice(items, func(i, j int) bool {
		// Parse both values before checking for errors
		valI, errI := o.parseFunc(items[i].captures[o.regexKey])
		valJ, errJ := o.parseFunc(items[j].captures[o.regexKey])
		if errI != nil && errJ != nil {
			items[i].err = errI
			items[j].err = errJ
			return true // Sort i to the top of the slice
		}
		if errI != nil {
			items[i].err = errI
			return true // Sort i to top of the slice
		}
		if errJ != nil {
			return false // Sort j to top of the slice
		}
		return o.compareFunc(valI, valJ)
	})

	// If there were errors, they are at the top of the slice.
	var errs error
	for i, it := range items {
		if it.err == nil {
			// No more errors, return the good items
			return items[i:], errs
		}
		errs = multierr.Append(errs, it.err)
	}

	// All items errored, clear the slice
	return []*item{}, errs
}

func SortNumeric(regexKey string, ascending bool) (Option, error) {
	return newRegexSortOption(regexKey,
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
	return newRegexSortOption(regexKey,
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
	return newRegexSortOption(regexKey,
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

type mtimeSortOption struct{}

type mtimeItem struct {
	mtime time.Time
	path  string
	item  *item
}

func (m mtimeSortOption) apply(items []*item) ([]*item, error) {
	mtimeItems := make([]mtimeItem, 0, len(items))
	var errs error
	for _, item := range items {
		path := item.value
		fi, err := os.Stat(path)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		mtimeItems = append(mtimeItems, mtimeItem{
			mtime: fi.ModTime(),
			path:  path,
			item:  item,
		})
	}

	sort.SliceStable(mtimeItems, func(i, j int) bool {
		// This checks if item i > j, in order to reverse the sort (most recently modified file is first in the list)
		return mtimeItems[i].mtime.After(mtimeItems[j].mtime)
	})

	filteredValues := make([]*item, 0, len(items))
	for _, mtimeItem := range mtimeItems {
		filteredValues = append(filteredValues, mtimeItem.item)
	}

	return filteredValues, errs
}

func SortMtime() Option {
	return mtimeSortOption{}
}
