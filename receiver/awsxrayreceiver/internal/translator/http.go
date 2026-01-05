// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventionsv118 "go.opentelemetry.io/otel/semconv/v1.18.0"
	conventionsv120 "go.opentelemetry.io/otel/semconv/v1.20.0"
	conventionsv125 "go.opentelemetry.io/otel/semconv/v1.25.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

func addHTTP(seg *awsxray.Segment, span ptrace.Span) {
	if seg.HTTP == nil {
		return
	}

	attrs := span.Attributes()
	if req := seg.HTTP.Request; req != nil {
		// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html#api-segmentdocuments-http
		addString(req.Method, string(conventionsv125.HTTPMethodKey), attrs)

		if req.ClientIP != nil {
			// since the ClientIP is not nil, this means that this segment is generated
			// by a server serving an incoming request
			attrs.PutStr(string(conventionsv120.HTTPClientIPKey), *req.ClientIP)
		}

		addString(req.UserAgent, string(conventionsv118.HTTPUserAgentKey), attrs)
		addString(req.URL, string(conventionsv125.HTTPURLKey), attrs)
		addBool(req.XForwardedFor, awsxray.AWSXRayXForwardedForAttribute, attrs)
	}

	if resp := seg.HTTP.Response; resp != nil {
		if resp.Status != nil {
			otStatus := tracetranslator.StatusCodeFromHTTP(*resp.Status)
			// in X-Ray exporter, the segment status is set:
			// first via the span attribute, string(conventions.HTTPStatusCodeKey)
			// then the span status. Since we are also setting the span attribute
			// below, the span status code here will not be actually used
			span.Status().SetCode(otStatus)
			attrs.PutInt(string(conventionsv125.HTTPStatusCodeKey), *resp.Status)
		}

		switch val := resp.ContentLength.(type) {
		case string:
			addString(&val, string(conventionsv125.HTTPResponseContentLengthKey), attrs)
		case float64:
			lengthPointer := int64(val)
			addInt64(&lengthPointer, string(conventionsv125.HTTPResponseContentLengthKey), attrs)
		}
	}
}
