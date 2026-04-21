// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"errors"
	"regexp"
)

func MatchValues(value string, regexp *regexp.Regexp) (map[string]any, error) {
	matches := regexp.FindStringSubmatch(value)
	if matches == nil {
		return nil, errors.New("regex pattern does not match")
	}

	parsedValues := map[string]any{}
	for i, subexp := range regexp.SubexpNames() {
		if i == 0 {
			// Skip whole match
			continue
		}
		if subexp != "" {
			parsedValues[subexp] = matches[i]
		}
	}
	return parsedValues, nil
}
