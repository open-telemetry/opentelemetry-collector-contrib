// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"strconv"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func writeDataStream(v *json.Visitor, idx elasticsearch.Index) {
	if !idx.IsDataStream() {
		return
	}
	_ = v.OnKey("data_stream")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "type", idx.Type)
	writeStringFieldSkipDefault(v, "dataset", idx.Dataset)
	writeStringFieldSkipDefault(v, "namespace", idx.Namespace)
	_ = v.OnObjectFinished()
}

func writeStringFieldSkipDefault(v *json.Visitor, key, value string) {
	if value == "" {
		return
	}
	_ = v.OnKey(key)
	_ = v.OnString(value)
}

func writeTimestampField(v *json.Visitor, key string, timestamp pcommon.Timestamp) {
	_ = v.OnKey(key)
	nsec := uint64(timestamp)
	msec := nsec / 1e6
	nsec -= msec * 1e6
	_ = v.OnString(strconv.FormatUint(msec, 10) + "." + strconv.FormatUint(nsec, 10))
}
