// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// SerializeProfile serializes a profile into the specified buffer
func SerializeProfile(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, profile pprofile.Profile, idx elasticsearch.Index, buf *bytes.Buffer) error {
	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	writeProfileSamples(v, idx, profile)

	return nil
}

func writeProfileSamples(v *json.Visitor, idx elasticsearch.Index, profile pprofile.Profile) {
	for i := 0; i < profile.Sample().Len(); i++ {
		sample := profile.Sample().At(i)

		for j := 0; j < sample.TimestampsUnixNano().Len(); j++ {
			t := sample.TimestampsUnixNano().At(j)

			_ = v.OnObjectStart(-1, structform.AnyType)
			writeDataStream(v, idx)
			writeTimestampField(v, "@timestamp", pcommon.Timestamp(t))

			_ = v.OnObjectFinished()
		}
	}
}
