// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func addSdkToResource(seg *awsxray.Segment, attrs pcommon.Map) {
	if seg.AWS != nil && seg.AWS.XRay != nil {
		xr := seg.AWS.XRay
		addString(xr.SDKVersion, string(conventions.TelemetrySDKVersionKey), attrs)
		if xr.SDK != nil {
			attrs.PutStr(string(conventions.TelemetrySDKNameKey), *xr.SDK)
			if seg.Cause != nil && len(seg.Cause.Exceptions) > 0 {
				// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c615d2db351929b99e46f7b427f39c12afe15b54/exporter/awsxrayexporter/translator/cause.go#L150
				// x-ray exporter only supports Java stack trace for now
				// TODO: Update this once the exporter is more flexible
				attrs.PutStr(string(conventions.TelemetrySDKLanguageKey), "java")
			} else {
				// sample *xr.SDK: "X-Ray for Go"
				sep := "for "
				sdkStr := *xr.SDK
				_, after, ok := strings.Cut(sdkStr, sep)
				if ok {
					attrs.PutStr(string(conventions.TelemetrySDKLanguageKey), after)
				}
			}
		}
	}
}
