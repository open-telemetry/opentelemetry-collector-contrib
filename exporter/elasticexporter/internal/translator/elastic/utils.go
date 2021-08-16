// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package elastic contains an opentelemetry-collector exporter
// for Elastic APM.
package elastic

import (
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
)

var (
	serviceNameInvalidRegexp = regexp.MustCompile("[^a-zA-Z0-9 _-]")
	labelKeyReplacer         = strings.NewReplacer(`.`, `_`, `*`, `_`, `"`, `_`)
)

func ifaceAttributeValue(v pdata.AttributeValue) interface{} {
	switch v.Type() {
	case pdata.AttributeValueTypeString:
		return truncate(v.StringVal())
	case pdata.AttributeValueTypeInt:
		return v.IntVal()
	case pdata.AttributeValueTypeDouble:
		return v.DoubleVal()
	case pdata.AttributeValueTypeBool:
		return v.BoolVal()
	}
	return nil
}

func cleanServiceName(name string) string {
	return serviceNameInvalidRegexp.ReplaceAllString(truncate(name), "_")
}

func cleanLabelKey(k string) string {
	return labelKeyReplacer.Replace(truncate(k))
}

func truncate(s string) string {
	const maxRunes = 1024
	var j int
	for i := range s {
		if j == maxRunes {
			return s[:i]
		}
		j++
	}
	return s
}
