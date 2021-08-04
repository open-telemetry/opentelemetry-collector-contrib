// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicexporter

import (
	"regexp"

	"go.opentelemetry.io/collector/model/pdata"
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
func (f *filter) filterIn(attributes pdata.AttributeMap) fields {
	returnValue := pdata.NewAttributeMap()

	attributes.Range(func(k string, v pdata.AttributeValue) bool {
		for _, regex := range f.regexes {
			if regex.MatchString(k) {
				returnValue.Insert(k, v)
				return true
			}
		}
		return true
	})
	returnValue.Sort()
	return newFields(returnValue)
}

// filterOut returns fields which don't match any of the filter regexes
func (f *filter) filterOut(attributes pdata.AttributeMap) fields {
	returnValue := pdata.NewAttributeMap()

	attributes.Range(func(k string, v pdata.AttributeValue) bool {
		for _, regex := range f.regexes {
			if regex.MatchString(k) {
				return true
			}
		}
		returnValue.Insert(k, v)
		return true
	})
	returnValue.Sort()
	return newFields(returnValue)
}
