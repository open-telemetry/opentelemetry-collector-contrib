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
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type filter struct {
	regexes []*regexp.Regexp
}

// Fields represents concatenated metadata
type Fields string

func newFilter(fields []string) (filter, error) {
	metadataRegexes := make([]*regexp.Regexp, len(fields))

	for i, field := range fields {
		regex, err := regexp.Compile(field)
		if err != nil {
			return filter{}, err
		}

		metadataRegexes[i] = regex
	}

	return filter{
		regexes: metadataRegexes,
	}, nil
}

// convertAttributeToString returns string which represents given pdata.AttributeValue
func (f *filter) convertAttributeToString(value pdata.AttributeValue) string {
	switch value.Type() {
	case pdata.AttributeValueSTRING:
		return value.StringVal()
	case pdata.AttributeValueBOOL:
		return strconv.FormatBool(value.BoolVal())
	case pdata.AttributeValueINT:
		return strconv.FormatInt(value.IntVal(), 10)
	case pdata.AttributeValueDOUBLE:
		return fmt.Sprintf("%g", value.DoubleVal())
	// For now only flat structure of the attributes is supported
	case pdata.AttributeValueMAP:
		return ""
	case pdata.AttributeValueARRAY:
		return ""
	case pdata.AttributeValueNULL:
		return ""
	default:
		return ""
	}
}

// filter returns map of strings which matches at least one of the filter regexes
func (f *filter) filter(attributes pdata.AttributeMap) map[string]string {
	returnValue := make(map[string]string)

	attributes.ForEach(func(k string, v pdata.AttributeValue) {
		for _, regex := range f.regexes {
			if regex.MatchString(k) {
				returnValue[k] = f.convertAttributeToString(v)
				return
			}
		}
	})
	return returnValue
}

// filterOut returns map of strings which doesn't match any of the filter regexes
func (f *filter) filterOut(attributes pdata.AttributeMap) map[string]string {
	returnValue := make(map[string]string)

	attributes.ForEach(func(k string, v pdata.AttributeValue) {
		for _, regex := range f.regexes {
			if regex.MatchString(k) {
				return
			}
		}
		returnValue[k] = f.convertAttributeToString(v)
	})
	return returnValue
}

// GetMetadata builds string which represents metadata in alphabetical order
func (f *filter) GetMetadata(attributes pdata.AttributeMap) Fields {
	attrs := f.filter(attributes)
	metadata := make([]string, 0, len(attrs))

	for k, v := range attrs {
		metadata = append(metadata, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(metadata)

	return Fields(strings.Join(metadata, ", "))
}
