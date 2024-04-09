// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type filter struct {
	regexes []*regexp.Regexp
}

func newFilter(flds []string) (filter, error) {
	metadataRegexes := make([]*regexp.Regexp, len(flds))

	for i, fld := range flds {
		regex, err := regexp.Compile(fld)
		if err != nil {
			return filter{}, err
		}

		metadataRegexes[i] = regex
	}

	return filter{
		regexes: metadataRegexes,
	}, nil
}

// filterIn returns fields which match at least one of the filter regexes
func (f *filter) filterIn(attributes pcommon.Map) fields {
	returnValue := pcommon.NewMap()

	attributes.Range(func(k string, v pcommon.Value) bool {
		for _, regex := range f.regexes {
			if regex.MatchString(k) {
				v.CopyTo(returnValue.PutEmpty(k))
				return true
			}
		}
		return true
	})

	return newFields(returnValue)
}

// filterOut returns fields which don't match any of the filter regexes
func (f *filter) filterOut(attributes pcommon.Map) fields {
	returnValue := pcommon.NewMap()

	attributes.Range(func(k string, v pcommon.Value) bool {
		for _, regex := range f.regexes {
			if regex.MatchString(k) {
				return true
			}
		}
		v.CopyTo(returnValue.PutEmpty(k))
		return true
	})

	return newFields(returnValue)
}
