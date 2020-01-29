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

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	semconventions "github.com/open-telemetry/opentelemetry-collector/translator/conventions"
	tracetranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace"
)

// HTTPData provides the shape for unmarshalling request and response data.
type HTTPData struct {
	Request  RequestData  `json:"request,omitempty"`
	Response ResponseData `json:"response,omitempty"`
}

// RequestData provides the shape for unmarshalling request data.
type RequestData struct {
	Method        string `json:"method,omitempty"`
	URL           string `json:"url,omitempty"` // http(s)://host/path
	ClientIP      string `json:"client_ip,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	XForwardedFor bool   `json:"x_forwarded_for,omitempty"`
	Traced        bool   `json:"traced,omitempty"`
}

// ResponseData provides the shape for unmarshalling response data.
type ResponseData struct {
	Status        int64 `json:"status,omitempty"`
	ContentLength int64 `json:"content_length,omitempty"`
}

func makeHTTP(span *tracepb.Span) (map[string]string, *HTTPData) {
	var (
		info           HTTPData
		filtered       = make(map[string]string)
		urlParts       = make(map[string]string)
		componentValue string
	)

	for key, value := range span.Attributes.AttributeMap {
		switch key {
		case semconventions.AttributeComponent:
			componentValue = value.GetStringValue().GetValue()
			filtered[key] = componentValue
		case semconventions.AttributeHTTPMethod:
			info.Request.Method = value.GetStringValue().GetValue()
		case semconventions.AttributeHTTPClientIP:
			info.Request.ClientIP = value.GetStringValue().GetValue()
			info.Request.XForwardedFor = true
		case semconventions.AttributeHTTPUserAgent:
			info.Request.UserAgent = value.GetStringValue().GetValue()
		case semconventions.AttributeHTTPStatusCode:
			info.Response.Status = value.GetIntValue()
		case semconventions.AttributeHTTPURL:
			urlParts[key] = value.GetStringValue().GetValue()
		case semconventions.AttributeHTTPScheme:
			urlParts[key] = value.GetStringValue().GetValue()
		case semconventions.AttributeHTTPHost:
			urlParts[key] = value.GetStringValue().GetValue()
		case semconventions.AttributeHTTPTarget:
			urlParts[key] = value.GetStringValue().GetValue()
		case semconventions.AttributeHTTPServerName:
			urlParts[key] = value.GetStringValue().GetValue()
		case semconventions.AttributeHostName:
			urlParts[key] = value.GetStringValue().GetValue()
		case semconventions.AttributeHTTPHostPort:
			urlParts[key] = value.GetStringValue().GetValue()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.GetIntValue(), 10)
			}
		case semconventions.AttributeNetPeerName:
			urlParts[key] = value.GetStringValue().GetValue()
		case semconventions.AttributeNetPeerPort:
			urlParts[key] = value.GetStringValue().GetValue()
			if len(urlParts[key]) == 0 {
				urlParts[key] = strconv.FormatInt(value.GetIntValue(), 10)
			}
		case semconventions.AttributeNetPeerIP:
			urlParts[key] = value.GetStringValue().GetValue()
		default:
			filtered[key] = value.GetStringValue().GetValue()
		}
	}

	if (componentValue != semconventions.ComponentTypeHTTP && componentValue != semconventions.ComponentTypeGRPC) ||
		info.Request.Method == "" {
		return filtered, nil
	}

	if tracepb.Span_SERVER == span.Kind {
		info.Request.URL = constructServerURL(componentValue, urlParts)
	} else {
		info.Request.URL = constructClientURL(componentValue, urlParts)
	}

	if info.Response.Status == 0 {
		info.Response.Status = int64(tracetranslator.HTTPStatusCodeFromOCStatus(span.Status.Code))
	}

	info.Response.ContentLength = extractResponseSizeFromEvents(span)

	return filtered, &info
}

func extractResponseSizeFromEvents(span *tracepb.Span) int64 {
	var size int64
	if span.TimeEvents != nil {
		for _, te := range span.TimeEvents.TimeEvent {
			anno := te.GetAnnotation()
			if anno != nil {
				attrMap := anno.Attributes.AttributeMap
				typeVal := attrMap[semconventions.AttributeMessageType]
				if typeVal != nil {
					if typeVal.GetStringValue().GetValue() == "RECEIVED" {
						sizeVal := attrMap[semconventions.AttributeMessageUncompressedSize]
						if sizeVal != nil {
							size = sizeVal.GetIntValue()
						}
					}
				}
			}
		}
	}
	return size
}

func constructClientURL(component string, urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md
	url, ok := urlParts[semconventions.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[semconventions.AttributeHTTPScheme]
	if !ok {
		if component == semconventions.ComponentTypeGRPC {
			scheme = "dns"
		} else {
			scheme = "http"
		}
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

func constructServerURL(component string, urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for server spans described in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/data-http.md
	url, ok := urlParts[semconventions.AttributeHTTPURL]
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := urlParts[semconventions.AttributeHTTPScheme]
	if !ok {
		if component == semconventions.ComponentTypeGRPC {
			scheme = "dns"
		} else {
			scheme = "http"
		}
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
