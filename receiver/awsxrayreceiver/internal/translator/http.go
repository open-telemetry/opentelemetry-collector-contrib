// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
)

func addHTTP(seg *awsxray.Segment, span ptrace.Span) {
	if seg.HTTP == nil {
		return
	}

	attrs := span.Attributes()
	if req := seg.HTTP.Request; req != nil {
		// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html#api-segmentdocuments-http
		if !metadata.ReceiverAwsxrayreceiverDontEmitV0HTTPConventionsFeatureGate.IsEnabled() {
			addString(req.Method, "http.method", attrs)
		}
		if metadata.ReceiverAwsxrayreceiverEmitV1HTTPConventionsFeatureGate.IsEnabled() {
			addString(req.Method, string(conventions.HTTPRequestMethodKey), attrs)
		}

		if req.ClientIP != nil {
			if !metadata.ReceiverAwsxrayreceiverDontEmitV0HTTPConventionsFeatureGate.IsEnabled() {
				attrs.PutStr("http.client_ip", *req.ClientIP)
			}
			if metadata.ReceiverAwsxrayreceiverEmitV1HTTPConventionsFeatureGate.IsEnabled() {
				attrs.PutStr(string(conventions.ClientAddressKey), *req.ClientIP)
			}
		}

		if !metadata.ReceiverAwsxrayreceiverDontEmitV0HTTPConventionsFeatureGate.IsEnabled() {
			addString(req.UserAgent, "http.user_agent", attrs)
			addString(req.URL, "http.url", attrs)
		}
		if metadata.ReceiverAwsxrayreceiverEmitV1HTTPConventionsFeatureGate.IsEnabled() {
			addString(req.UserAgent, string(conventions.UserAgentOriginalKey), attrs)
			addString(req.URL, string(conventions.URLFullKey), attrs)
		}
		addBool(req.XForwardedFor, awsxray.AWSXRayXForwardedForAttribute, attrs)
	}

	if resp := seg.HTTP.Response; resp != nil {
		if resp.Status != nil {
			otStatus := tracetranslator.StatusCodeFromHTTP(*resp.Status)
			span.Status().SetCode(otStatus)
			if !metadata.ReceiverAwsxrayreceiverDontEmitV0HTTPConventionsFeatureGate.IsEnabled() {
				attrs.PutInt("http.status_code", *resp.Status)
			}
			if metadata.ReceiverAwsxrayreceiverEmitV1HTTPConventionsFeatureGate.IsEnabled() {
				attrs.PutInt(string(conventions.HTTPResponseStatusCodeKey), *resp.Status)
			}
		}

		switch val := resp.ContentLength.(type) {
		case string:
			if !metadata.ReceiverAwsxrayreceiverDontEmitV0HTTPConventionsFeatureGate.IsEnabled() {
				addString(&val, "http.response_content_length", attrs)
			}
			if metadata.ReceiverAwsxrayreceiverEmitV1HTTPConventionsFeatureGate.IsEnabled() {
				addString(&val, string(conventions.HTTPResponseBodySizeKey), attrs)
			}
		case float64:
			lengthPointer := int64(val)
			if !metadata.ReceiverAwsxrayreceiverDontEmitV0HTTPConventionsFeatureGate.IsEnabled() {
				addInt64(&lengthPointer, "http.response_content_length", attrs)
			}
			if metadata.ReceiverAwsxrayreceiverEmitV1HTTPConventionsFeatureGate.IsEnabled() {
				addInt64(&lengthPointer, string(conventions.HTTPResponseBodySizeKey), attrs)
			}
		}
	}
}
