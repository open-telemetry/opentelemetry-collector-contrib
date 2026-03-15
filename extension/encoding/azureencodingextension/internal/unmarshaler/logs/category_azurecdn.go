// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"encoding/json"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// Non-SemConv attributes that are used for common Azure Log Record fields
const (
	// OpenTelemetry attribute name for point of presence (POP),
	// from `pop` field in Azure Log Record
	attributeAzurePop = "azure.cdn.edge.name"

	// OpenTelemetry attribute name for time to first byte (TTFB) in milliseconds
	// from when Microsoft service (CDN, Front Door, etc) receives the
	// request to the time the first byte gets sent to the client
	attributeAzureTimeToFirstByte = "azure.time_to_first_byte"

	// OpenTelemetry attribute name for cache status,
	// holds the result of the cache hit/miss at the point of presence (POP)
	attributeAzureCacheStatus = "azure.cdn.cache.outcome"

	// OpenTelemetry attribute name for server name indication (SNI) value
	// At the moment SemConv does not have dedicated attribute for SNI
	attributeTLSServerName = "tls.server.name"
)

const (
	noError = "NoError"
	naValue = "N/A"
)

// Azure CDN Access Log (AzureCdnAccessLog) and Azure Front Door Access Log (FrontDoorAccessLog)
// shares 90% of the "properties" fields
// So, to simplify support - we will be using combined structure called `azureHTTPAccessLog`
// that is capable to parse "properties" for both categories

// See https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/cdn/monitoring-and-access-log.md
// See https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/frontdoor/monitor-front-door.md
// See https://learn.microsoft.com/en-us/azure/frontdoor/monitor-front-door?pivots=front-door-standard-premium#access-log.
type azureHTTPAccessLog struct {
	azureLogRecordBase

	Properties struct {
		TrackingReference string      `json:"trackingReference"`
		HTTPMethod        string      `json:"httpMethod"`
		HTTPVersion       string      `json:"httpVersion"`
		RequestURI        string      `json:"requestUri"`
		SNI               string      `json:"sni"`
		RequestBytes      json.Number `json:"requestBytes"`  // int
		ResponseBytes     json.Number `json:"responseBytes"` // int
		UserAgent         string      `json:"userAgent"`
		ClientIP          string      `json:"clientIp"`
		ClientPort        json.Number `json:"clientPort"` // int
		SocketIP          string      `json:"socketIp"`
		TimeToFirstByte   json.Number `json:"timeToFirstByte"` // float
		TimeTaken         json.Number `json:"timeTaken"`       // float
		RequestProtocol   string      `json:"requestProtocol"`
		SecurityProtocol  string      `json:"securityProtocol"`
		HTTPStatusCode    json.Number `json:"httpStatusCode"` // int
		Pop               string      `json:"pop"`
		CacheStatus       string      `json:"cacheStatus"`
		ErrorInfo         string      `json:"ErrorInfo"`
		Endpoint          string      `json:"endpoint"`
		Result            string      `json:"result"`
		// Fields from FrontDoorAccessLog only
		HostName       *string `json:"hostName"`
		SecurityCipher string  `json:"securityCipher"`
		SecurityCurves string  `json:"securityCurves"`
		OriginIP       string  `json:"originIp"`
		// Fields from AzureCdnAccessLog only
		IsReceivedFromClient *bool   `json:"isReceivedFromClient"`
		BackendHostname      *string `json:"backendHostname"`
	} `json:"properties"`
}

func (r *azureHTTPAccessLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPRequestSizeKey), r.Properties.RequestBytes)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPResponseSizeKey), r.Properties.ResponseBytes)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.ClientPortKey), r.Properties.ClientPort)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPResponseStatusCodeKey), r.Properties.HTTPStatusCode)
	unmarshaler.AttrPutFloatNumberIf(attrs, attributeAzureTimeToFirstByte, r.Properties.TimeToFirstByte)
	unmarshaler.AttrPutFloatNumberIf(attrs, attributeAzureRequestDuration, r.Properties.TimeTaken)
	unmarshaler.AttrPutURLParsed(attrs, r.Properties.RequestURI)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.AzureServiceRequestIDKey), r.Properties.TrackingReference)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HTTPRequestMethodKey), r.Properties.HTTPMethod)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolVersionKey), r.Properties.HTTPVersion)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolNameKey), strings.ToLower(r.Properties.RequestProtocol))
	if r.Properties.SNI != "" && r.Properties.SNI != naValue {
		unmarshaler.AttrPutStrIf(attrs, attributeTLSServerName, r.Properties.SNI)
	}
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserAgentOriginalKey), r.Properties.UserAgent)
	unmarshaler.AttrPutHostPortIf(attrs, string(conventions.ClientAddressKey), string(conventions.ClientPortKey), r.Properties.ClientIP)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkPeerAddressKey), r.Properties.SocketIP)
	unmarshaler.AttrPutStrIf(attrs, attributeAzurePop, r.Properties.Pop)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureCacheStatus, r.Properties.CacheStatus)
	attrPutTLSProtoIf(attrs, r.Properties.SecurityProtocol)
	if r.Properties.ErrorInfo != noError {
		unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionTypeKey), r.Properties.ErrorInfo)
	}
	if r.Properties.Result != "" && r.Properties.Result != naValue {
		body.SetStr(r.Properties.Result)
	}
	// Fields from FrontDoorAccessLog only
	unmarshaler.AttrPutStrIf(attrs, string(conventions.TLSCurveKey), r.Properties.SecurityCurves)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.TLSCipherKey), r.Properties.SecurityCipher)
	if r.Properties.BackendHostname != nil {
		// "BackendHostname" is the field represents the hostname of the backend, used in AzureCdnAccessLog
		unmarshaler.AttrPutHostPortIf(attrs, string(conventions.ServerAddressKey), string(conventions.ServerPortKey), *r.Properties.BackendHostname)
	} else {
		// "OriginIP" is the field represents the origin server IP, used in FrontDoorAccessLog
		unmarshaler.AttrPutHostPortIf(attrs, string(conventions.ServerAddressKey), string(conventions.ServerPortKey), r.Properties.OriginIP)
	}
	// "Endpoint" is the domain name of the Azure Front Door edge endpoint
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkLocalAddressKey), r.Properties.Endpoint)
	// "HostName" is the the host name in the request from client,
	// this is actually a "Host" HTTP header value
	unmarshaler.AttrPutStrPtrIf(attrs, attributeHTTPHeaderHost, r.Properties.HostName)

	if r.Properties.IsReceivedFromClient != nil {
		direction := "transmit"
		if *r.Properties.IsReceivedFromClient {
			direction = "receive"
		}
		attrs.PutStr(string(conventions.NetworkIODirectionKey), direction)
	}

	return nil
}
