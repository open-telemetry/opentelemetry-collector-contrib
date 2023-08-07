// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"

import (
	"fmt"
	"regexp"
)

type item struct {
	value    string
	captures map[string]string

	// Used when an Option is unable to interpret the value.
	// For example, a numeric sort may fail to parse the value as a number.
	err error
}

func newItem(value string, regex *regexp.Regexp) (*item, error) {
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
