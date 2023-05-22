// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
