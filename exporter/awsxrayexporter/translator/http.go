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

package translator

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
)

func makeHTTP(span pdata.Span) (map[string]string, *awsxray.HTTPData) {
	var (
		info = awsxray.HTTPData{
			Request:  &awsxray.RequestData{},
			Response: &awsxray.ResponseData{},
		}
		filtered = make(map[string]string)
		urlParts = make(map[string]string)
	)

	if span.Attributes().Len() == 0 {
		return filtered, nil
	}

	hasHTTP := false

	span.Attributes().ForEach(func(key string, value pdata.AttributeValue) {
		switch key {
		case semconventions.AttributeHTTPMethod:
			info.Request.Method = awsxray.String(value.StringVal())
			hasHTTP = true
		case semconventions.AttributeHTTPClientIP:
			info.Request.ClientIP = awsxray.String(value.StringVal())
			info.Request.XForwardedFor = aws.Bool(true)
			hasHTTP = true
		case semconventions.AttributeHTTPUserAgent:
			info.Request.UserAgent = awsxray.String(value.StringVal())
			hasHTTP = true
		case semconventions.AttributeHTTPStatusCode:
			info.Response.Status = aws.Int64(value.IntVal())
			hasHTTP = true
		case semconventions.AttributeHTTPURL:
			urlParts[key] = value.StringVal()
			hasHTTP = true
		case semconventions.AttributeHTTPScheme:
			urlParts[key] = value.StringVal()
			hasHTTP = true
		case semconventions.AttributeHTTPHost:
			urlParts[key] = value.StringVal()
			hasHTTP = true
		case semconventions.AttributeHTTPTarget:
			urlParts[key] = value.StringVal()
			hasHTTP = true
		case semconventions.AttributeHTTPServerName:
			urlParts[key] = value.StringVal()
			hasHTTP = true
		case semconventions.AttributeHTTPHostPort:
			urlParts[key] = value.StringVal()
			hasHTTP = true
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.IntVal(), 10)
			}
		case semconventions.AttributeHostName:
			urlParts[key] = value.StringVal()
		case semconventions.AttributeNetPeerName:
			urlParts[key] = value.StringVal()
		case semconventions.AttributeNetPeerPort:
			urlParts[key] = value.StringVal()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.IntVal(), 10)
			}
		case semconventions.AttributeNetPeerIP:
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if info.Request.ClientIP == nil {
				info.Request.ClientIP = awsxray.String(value.StringVal())
			}
			urlParts[key] = value.StringVal()
		default:
			filtered[key] = value.StringVal()
		}
	})

	if !hasHTTP {
		// Didn't have any HTTP-specific information so don't need to fill it in segment
		return filtered, nil
	}

	if span.Kind() == pdata.SpanKindSERVER {
		info.Request.URL = awsxray.String(constructServerURL(urlParts))
	} else {
		info.Request.URL = awsxray.String(constructClientURL(urlParts))
	}

	if !span.Status().IsNil() && info.Response.Status == nil {
		// TODO(anuraaga): Replace with direct translator of StatusCode without casting to int
		info.Response.Status = aws.Int64(int64(tracetranslator.HTTPStatusCodeFromOCStatus(int32(span.Status().Code()))))
	}

	info.Response.ContentLength = aws.Int64(extractResponseSizeFromEvents(span))

	return filtered, &info
}

func extractResponseSizeFromEvents(span pdata.Span) int64 {
	// Support insrumentation that sets response size in span or as an event.
	size := extractResponseSizeFromAttributes(span.Attributes())
	if size != 0 {
		return size
	}
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		size = extractResponseSizeFromAttributes(event.Attributes())
		if size != 0 {
			return size
		}
	}
	return size
}

func extractResponseSizeFromAttributes(attributes pdata.AttributeMap) int64 {
	typeVal, ok := attributes.Get(semconventions.AttributeMessageType)
	if ok && typeVal.StringVal() == "RECEIVED" {
		if sizeVal, ok := attributes.Get(semconventions.AttributeMessageUncompressedSize); ok {
			return sizeVal.IntVal()
		}
	}
	return 0
}

func constructClientURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md
	url, ok := urlParts[semconventions.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[semconventions.AttributeHTTPScheme]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[semconventions.AttributeHTTPHost]
	if !ok {
		host, ok = urlParts[semconventions.AttributeNetPeerName]
		if !ok {
			host = urlParts[semconventions.AttributeNetPeerIP]
		}
		port, ok = urlParts[semconventions.AttributeNetPeerPort]
		if !ok {
			port = ""
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[semconventions.AttributeHTTPTarget]
	if ok {
		url += target
	} else {
		url += "/"
	}
	return url
}

func constructServerURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for server spans described in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md
	url, ok := urlParts[semconventions.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[semconventions.AttributeHTTPScheme]
	if !ok {
		scheme = "http"
	}
	port := ""
	host, ok := urlParts[semconventions.AttributeHTTPHost]
	if !ok {
		host, ok = urlParts[semconventions.AttributeHTTPServerName]
		if !ok {
			host = urlParts[semconventions.AttributeHostName]
		}
		port, ok = urlParts[semconventions.AttributeHTTPHostPort]
		if !ok {
			port = ""
		}
	}
	url = scheme + "://" + host
	if len(port) > 0 && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		url += ":" + port
	}
	target, ok := urlParts[semconventions.AttributeHTTPTarget]
	if ok {
		url += target
	} else {
		url += "/"
	}
	return url
}
