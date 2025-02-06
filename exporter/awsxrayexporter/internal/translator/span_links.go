// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func makeSpanLinks(links ptrace.SpanLinkSlice, skipTimestampValidation bool) ([]awsxray.SpanLinkData, error) {
	var spanLinkDataArray []awsxray.SpanLinkData

	for i := 0; i < links.Len(); i++ {
		var spanLinkData awsxray.SpanLinkData
		link := links.At(i)

		spanID := link.SpanID().String()
		traceID, err := convertToAmazonTraceID(link.TraceID(), skipTimestampValidation)
		if err != nil {
			return nil, err
		}

		spanLinkData.SpanID = &spanID
		spanLinkData.TraceID = &traceID

		if link.Attributes().Len() > 0 {
			spanLinkData.Attributes = make(map[string]any)

			link.Attributes().Range(func(k string, v pcommon.Value) bool {
				spanLinkData.Attributes[k] = v.AsRaw()
				return true
			})
		}

		spanLinkDataArray = append(spanLinkDataArray, spanLinkData)
	}

	return spanLinkDataArray, nil
}
