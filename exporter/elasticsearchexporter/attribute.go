// Copyright The OpenTelemetry Authors
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

// Package elasticsearchexporter contains an opentelemetry-collector exporter
// for Elasticsearch.
package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import "go.opentelemetry.io/collector/pdata/pcommon"

// dynamic index attribute key constants
const (
	indexPrefix = "elasticsearch.index.prefix"
	indexSuffix = "elasticsearch.index.suffix"
)

// resource is higher priotized than record attribute
type attrGetter interface {
	Attributes() pcommon.Map
}

// retrieve attribute out of resource and record (span or log, if not found in resource)
func getFromBothResourceAndAttribute(name string, resource attrGetter, record attrGetter) string {
	var str string
	val, exist := resource.Attributes().Get(name)
	if !exist {
		val, exist = record.Attributes().Get(name)
	}
	if exist {
		str = val.AsString()
	}
	return str
}
