// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"

func IndexDownsampledEvent(event StackTraceEvent, indexSuffix string, pushData func(any, string, string) error) error {
	return serializer.DownsampleEvent(event.Count, indexSuffix, func(count uint16, index string) error {
		event.Count = count
		return pushData(event, "", index)
	})
}
