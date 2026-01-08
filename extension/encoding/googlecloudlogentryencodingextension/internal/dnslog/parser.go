// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Find more information about Cloud dns logs at:
// https://docs.cloud.google.com/dns/docs/monitoring#dns-log-record-format
package dnslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/dnslog"

import (
	"errors"
	"fmt"
	"strings"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

const (
	CloudDNSQueryLogSuffix = "dns.googleapis.com%2Fdns_queries"

	// Query related attributes. Ref: https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.2
	// gcpDNSQueryType holds the DNS query type
	gcpDNSQueryType = "dns.question.type" // TBD in SemConv

	// Response related attributes
	// gcpDNSResponseCode holds the DNS response code
	gcpDNSResponseCode = "dns.response_code" // TBD in SemConv
	// gcpDNSAliasQueryResponseCode holds the response code for alias queries
	gcpDNSAliasQueryResponseCode = "gcp.dns.alias_query.response.code"
	// gcpDNSAuthAnswer indicates whether the response is authoritative
	gcpDNSAuthAnswer = "gcp.dns.auth_answer" // Ref: https://datatracker.ietf.org/doc/html/rfc1035
	// gcpDNSAnswerData holds DNS answer in presentation format
	gcpDNSAnswerData = "dns.answer.data" // TBD in SemConv

	// Network related attributes
	// gcpDNSClientVPCNetwork holds the client network name where the DNS query originated
	gcpDNSClientVPCNetwork = "gcp.dns.client.vpc.name"
	// gcpDNSClientType holds the client type of the DNS query
	gcpDNSClientType = "gcp.dns.client.type"

	// Server related attributes
	// gcpDNSServerName holds the name name of Google Cloud instance that is responsible of resolving the query
	gcpDNSServerName = "gcp.dns.server.name"
	// gcpDNServerType holds the type of target resolving the DNS query
	gcpDNServerType = "gcp.dns.server.type"

	// Performance and error related attributes
	// gcpDNSServerLatency holds the server-side latency in seconds
	gcpDNSServerLatency = "gcp.dns.server.latency"
	// gcpDNSEgressError holds Egress proxy error, the actual error as received from the on-premises DNS server
	gcpDNSEgressError = "gcp.dns.egress.error"
	// gcpDNSHealthyIPs holds addresses in the ResourceRecordSet that are known to be HEALTHY.
	gcpDNSHealthyIPs = "gcp.dns.healthy.ips"
	// gcpDNSUnhealthyIPs holds addresses in the ResourceRecordSet that are known to be UNHEALTHY.
	gcpDNSUnhealthyIPs = "gcp.dns.unhealthy.ips"

	// DNS feature related attributes
	// gcpDNSDNS64Translated indicates whether DNS64 translation was applied
	gcpDNSDNS64Translated = "gcp.dns.dns64.translated"

	// VM instance related attributes
	// gcpProjectID holds the Google Cloud project ID of the network from which the query was sent
	gcpProjectID = "gcp.project.id"
)

type dnslog struct {
	AliasQueryResponseCode string   `json:"alias_query_response_code"`
	AuthAnswer             *bool    `json:"authAnswer"`
	DestinationIP          string   `json:"destinationIP"`
	DNS64Translated        *bool    `json:"dns64Translated"`
	EgressError            string   `json:"egressError"`
	HealthyIps             string   `json:"healthyIps"`
	Location               string   `json:"location"`
	Protocol               string   `json:"protocol"`
	ProjectID              string   `json:"project_id"`
	QueryName              string   `json:"queryName"`
	QueryType              string   `json:"queryType"`
	Rdata                  string   `json:"rdata"`
	ResponseCode           string   `json:"responseCode"`
	ServerLatency          *float64 `json:"serverLatency"`
	SourceIP               string   `json:"sourceIP"`
	SourceNetwork          string   `json:"sourceNetwork"`
	SourceType             string   `json:"source_type"`
	TargetName             string   `json:"target_name"`
	TargetType             string   `json:"target_type"`
	UnhealthyIps           string   `json:"unhealthyIps"`
	VMInstanceID           *int64   `json:"vmInstanceId"`
	VMInstanceName         string   `json:"vmInstanceName"`
	VMProjectID            string   `json:"vmProjectId"`
	VMZoneName             string   `json:"vmZoneName"`
}

func handleQueryAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(string(conventions.DNSQuestionNameKey), log.QueryName, attr)
	shared.PutStr(gcpDNSQueryType, log.QueryType, attr) // TBD in SemConv
}

func handleResponseAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(gcpDNSResponseCode, log.ResponseCode, attr) // TBD in SemConv
	shared.PutStr(gcpDNSAliasQueryResponseCode, log.AliasQueryResponseCode, attr)
	shared.PutBool(gcpDNSAuthAnswer, log.AuthAnswer, attr)
	shared.PutStr(gcpDNSAnswerData, log.Rdata, attr) // TBD in SemConv
}

func handleNetworkAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(string(conventions.ServerAddressKey), log.DestinationIP, attr)
	shared.PutStr(gcpDNSClientVPCNetwork, log.SourceNetwork, attr)
	shared.PutStr(gcpDNSClientType, log.SourceType, attr)
	shared.PutStr(string(conventions.ClientAddressKey), log.SourceIP, attr)
	shared.PutStr(string(conventions.NetworkTransportKey), strings.ToLower(log.Protocol), attr)
	shared.PutStr(string(conventions.CloudRegionKey), log.Location, attr)
}

func handleTargetAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(gcpDNSServerName, log.TargetName, attr)
	shared.PutStr(gcpDNServerType, log.TargetType, attr)
}

func handlePerformanceAndErrorAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutDouble(gcpDNSServerLatency, log.ServerLatency, attr)
	shared.PutStr(gcpDNSEgressError, log.EgressError, attr)
	shared.PutStr(gcpDNSHealthyIPs, log.HealthyIps, attr)
	shared.PutStr(gcpDNSUnhealthyIPs, log.UnhealthyIps, attr)
}

func handleDNSFeatureAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutBool(gcpDNSDNS64Translated, log.DNS64Translated, attr)
}

func handleVMInstanceAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutInt(string(conventions.HostIDKey), log.VMInstanceID, attr)
	shared.PutStr(string(conventions.HostNameKey), log.VMInstanceName, attr)
	shared.PutStr(gcpProjectID, log.VMProjectID, attr)
	shared.PutStr(string(conventions.CloudAvailabilityZoneKey), log.VMZoneName, attr)
}

func ParsePayloadIntoAttributes(payload []byte, attr pcommon.Map) error {
	var log *dnslog
	if err := gojson.Unmarshal(payload, &log); err != nil {
		return fmt.Errorf("failed to unmarshal DNS log: %w", err)
	}

	if log == nil {
		return errors.New("DNS cannot be nil after detecting payload as DNS log")
	}

	handleQueryAttributes(log, attr)
	handleResponseAttributes(log, attr)
	handleNetworkAttributes(log, attr)
	handleTargetAttributes(log, attr)
	handlePerformanceAndErrorAttributes(log, attr)
	handleDNSFeatureAttributes(log, attr)
	handleVMInstanceAttributes(log, attr)

	return nil
}
