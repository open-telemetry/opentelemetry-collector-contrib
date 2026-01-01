// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

const (
	// OpenTelemetry attribute name for the operations mode of the WAF policy
	attributeSecurityRuleRulesetModeKey = "security_rule.ruleset.mode"

	// OpenTelemetry attribute name for the action taken on the request
	attributeSecurityRuleActionKey = "security_rule.action"

	// OpenTelemetry attribute name for the unique ID to identify the health probe request
	attributeAzureFrontDoorHealthProbeID = "azure.frontdoor.health_probe.id"

	// OpenTelemetry attribute name for the unique ID to identify the health probe request
	attributeAzureFrontDoorHealthOriginName = "azure.frontdoor.health_probe.origin.name"

	// OpenTelemetry attribute name for the time from when the Azure Front Door edge sent
	// the health probe request to the origin to when the origin sent the last response to Azure Front Door.
	attributeAzureFrontDoorHealthTotalLatency = "azure.frontdoor.health_probe.origin.latency.total"

	// OpenTelemetry attribute name for the time spent setting up the TCP connection
	// to send the HTTP probe request to the origin
	attributeAzureFrontDoorHealthConnLatency = "azure.frontdoor.health_probe.origin.latency.connection"

	// OpenTelemetry attribute name for the time spent on DNS resolution
	attributeAzureFrontDoorHealthDNSLatency = "azure.frontdoor.health_probe.origin.latency.dns"
)

// NOTE: For "FrontDoorAccessLog" category - see "azureHTTPAccessLog" struct in category_azurecdn.go

// See https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/frontdoor/monitor-front-door.md#health-probe-log
type frontDoorHealthProbeLog struct {
	azureLogRecordBase

	Properties struct {
		HealthProbeID     string      `json:"healthProbeId"`
		Pop               string      `json:"POP"`
		HTTPVerb          string      `json:"httpVerb"`
		Result            string      `json:"result"`
		HTTPStatusCode    json.Number `json:"httpStatusCode"` // int
		ProbeURL          string      `json:"probeURL"`
		OriginName        string      `json:"originName"`
		OriginIP          string      `json:"originIP"`
		TotalLatency      json.Number `json:"totalLatencyMilliseconds"`      // int, ms
		ConnectionLatency json.Number `json:"connectionLatencyMilliseconds"` // int, ms
		DNSLatency        json.Number `json:"DNSLatencyMicroseconds"`        // int, us
	} `json:"properties"`
}

func (r *frontDoorHealthProbeLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureFrontDoorHealthProbeID, r.Properties.HealthProbeID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzurePop, r.Properties.Pop)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HTTPRequestMethodKey), r.Properties.HTTPVerb)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPResponseStatusCodeKey), r.Properties.HTTPStatusCode)
	unmarshaler.AttrPutURLParsed(attrs, r.Properties.ProbeURL)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureFrontDoorHealthOriginName, r.Properties.OriginName)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureFrontDoorHealthTotalLatency, r.Properties.TotalLatency)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureFrontDoorHealthConnLatency, r.Properties.ConnectionLatency)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureFrontDoorHealthDNSLatency, r.Properties.DNSLatency)
	unmarshaler.AttrPutHostPortIf(attrs, string(conventions.ServerAddressKey), string(conventions.ServerPortKey), r.Properties.OriginIP)
	body.SetStr(r.Properties.Result)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/web-application-firewall/afds/waf-front-door-monitor?pivots=front-door-standard-premium#waf-logs
type frontDoorWAFLog struct {
	azureLogRecordBase

	Properties struct {
		ClientIP          string      `json:"clientIP"`
		ClientPort        json.Number `json:"clientPort"` // int
		SocketIP          string      `json:"socketIP"`
		RequestURI        string      `json:"requestUri"`
		RuleName          string      `json:"ruleName"`
		Policy            string      `json:"policy"`
		Action            string      `json:"action"`
		Host              string      `json:"host"`
		TrackingReference string      `json:"trackingReference"`
		PolicyMode        string      `json:"policyMode"`
	} `json:"properties"`
}

func (r *frontDoorWAFLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.ClientPortKey), r.Properties.ClientPort)
	unmarshaler.AttrPutURLParsed(attrs, r.Properties.RequestURI)
	unmarshaler.AttrPutHostPortIf(attrs, string(conventions.ClientAddressKey), string(conventions.ClientPortKey), r.Properties.ClientIP)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkPeerAddressKey), r.Properties.SocketIP)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.AzureServiceRequestIDKey), r.Properties.TrackingReference)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderHost, r.Properties.Host)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.SecurityRuleRulesetNameKey), r.Properties.Policy)
	unmarshaler.AttrPutStrIf(attrs, attributeSecurityRuleRulesetModeKey, r.Properties.PolicyMode)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.SecurityRuleNameKey), r.Properties.RuleName)
	unmarshaler.AttrPutStrIf(attrs, attributeSecurityRuleActionKey, r.Properties.Action)

	return nil
}
