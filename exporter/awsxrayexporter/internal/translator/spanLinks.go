// Copyright 2019, OpenTelemetry Authors
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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func makeSpanLinks(links ptrace.SpanLinkSlice) ([]awsxray.SpanLinkData, error) {
	var spanLinkDataArray []awsxray.SpanLinkData

	for i := 0; i < links.Len(); i++ {
		var spanLinkData awsxray.SpanLinkData
		var link = links.At(i)

		var spanID = link.SpanID().String()
		traceID, err := convertToAmazonTraceID(link.TraceID())

		if err != nil {
			return nil, err
		}

		spanLinkData.SpanID = &spanID
		spanLinkData.TraceID = &traceID

		if links.Len() > 0 {
			spanLinkData.Attributes = make(map[string]interface{})
		}

		link.Attributes().Range(func(k string, v pcommon.Value) bool {
			spanLinkData.Attributes[k] = v.AsRaw()
			return true
		})

		spanLinkDataArray = append(spanLinkDataArray, spanLinkData)
	}

	return spanLinkDataArray, nil
}
