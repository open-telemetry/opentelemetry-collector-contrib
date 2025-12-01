// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Find more information about Cloud dns logs at:
// https://docs.cloud.google.com/dns/docs/monitoring#dns-log-record-format
package dnslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/dnslog"

import (
	"fmt"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

const (
	CloudDNSQueryLogSuffix = "dns.googleapis.com%2Fdns_queries"

	// Query related attributes. Ref: https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.2
	// gcpDNSQueryName holds the DNS query name
	gcpDNSQueryName = "gcp.dns.query.name"
	// gcpDNSQueryType holds the DNS query type
	gcpDNSQueryType = "gcp.dns.query.type"

	// Response related attributes
	// gcpDNSResponseCode holds the DNS response code
	gcpDNSResponseCode = "gcp.dns.response.code"
	// gcpDNSAliasQueryResponseCode holds the response code for alias queries
	gcpDNSAliasQueryResponseCode = "gcp.dns.alias_query.response.code"
	// gcpDNSAuthAnswer indicates whether the response is authoritative
	gcpDNSAuthAnswer = "gcp.dns.auth_answer" // Ref: https://datatracker.ietf.org/doc/html/rfc1035
	// gcpDNSRdata holds DNS answer in presentation format
	gcpDNSRdata = "gcp.dns.rdata"

	// Network related attributes
	// gcpDNSDestinationIP holds target IP address
	gcpDNSDestinationIP = "gcp.dns.destination_ip"
	// gcpDNSSourceNetwork holds the source network name
	gcpDNSSourceNetwork = "gcp.dns.source.network"
	// gcpDNSSourceType holds the source type of the DNS query
	gcpDNSSourceType = "gcp.dns.source.type"

	// Target related attributes
	// gcpDNSTargetName holds the target name
	gcpDNSTargetName = "gcp.dns.target.name"
	// gcpDNSTargetType holds the type of target resolving the DNS query
	gcpDNSTargetType = "gcp.dns.target.type"

	// Performance and error related attributes
	// gcpDNSServerLatency holds the server-side latency in seconds
	gcpDNSServerLatency = "gcp.dns.server_latency"
	// gcpDNSEgressError holds Egress proxy error, the actual error as received from the on-premises DNS server
	gcpDNSEgressError = "gcp.dns.egress_error"
	// gcpDNSHealthyIPs holds addresses in the ResourceRecordSet that are known to be HEALTHY.
	gcpDNSHealthyIPs = "gcp.dns.healthy_ips"
	// gcpDNSUnhealthyIPs holds addresses in the ResourceRecordSet that are known to be UNHEALTHY.
	gcpDNSUnhealthyIPs = "gcp.dns.unhealthy_ips"

	// DNS feature related attributes
	// gcpDNSDNS64Translated indicates whether DNS64 translation was applied
	gcpDNSDNS64Translated = "gcp.dns.dns64_translated"

	// VM instance related attributes
	// gcpDNSVMInstanceID holds the Compute Engine VM instance ID as an integer
	gcpDNSVMInstanceID = "gcp.dns.vm.instance.id"
	// gcpDNSVMInstanceIDString holds the Compute Engine VM instance ID as a string
	gcpDNSVMInstanceIDString = "gcp.dns.vm.instance.id_string"
	// gcpDNSVMInstanceName holds the VM instance name
	gcpDNSVMInstanceName = "gcp.dns.vm.instance.name"
	// gcpDNSVMProjectID holds the Google Cloud project ID of the network from which the query was sent
	gcpDNSVMProjectID = "gcp.dns.vm.project_id"
	// gcpDNSVMZoneName holds the name of the VM zone from which the query was sent
	gcpDNSVMZoneName = "gcp.dns.vm.zone"
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
	VMInstanceIDStr        string   `json:"vmInstanceIdString"`
	VMInstanceName         string   `json:"vmInstanceName"`
	VMProjectID            string   `json:"vmProjectId"`
	VMZoneName             string   `json:"vmZoneName"`
}

func handleQueryAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(gcpDNSQueryName, log.QueryName, attr)
	shared.PutStr(gcpDNSQueryType, log.QueryType, attr)
}

func handleResponseAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(gcpDNSResponseCode, log.ResponseCode, attr)
	shared.PutStr(gcpDNSAliasQueryResponseCode, log.AliasQueryResponseCode, attr)
	shared.PutBool(gcpDNSAuthAnswer, log.AuthAnswer, attr)
	shared.PutStr(gcpDNSRdata, log.Rdata, attr)
}

func handleNetworkAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(gcpDNSDestinationIP, log.DestinationIP, attr)
	shared.PutStr(gcpDNSSourceNetwork, log.SourceNetwork, attr)
	shared.PutStr(gcpDNSSourceType, log.SourceType, attr)
	shared.PutStr(string(semconv.ClientAddressKey), log.SourceIP, attr)
	shared.PutStr(string(semconv.NetworkTransportKey), log.Protocol, attr)
	shared.PutStr(string(semconv.CloudRegionKey), log.Location, attr)
}

func handleTargetAttributes(log *dnslog, attr pcommon.Map) {
	shared.PutStr(gcpDNSTargetName, log.TargetName, attr)
	shared.PutStr(gcpDNSTargetType, log.TargetType, attr)
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
	shared.PutInt(gcpDNSVMInstanceID, log.VMInstanceID, attr)
	shared.PutStr(gcpDNSVMInstanceIDString, log.VMInstanceIDStr, attr)
	shared.PutStr(gcpDNSVMInstanceName, log.VMInstanceName, attr)
	shared.PutStr(gcpDNSVMProjectID, log.VMProjectID, attr)
	shared.PutStr(gcpDNSVMZoneName, log.VMZoneName, attr)
}

func ParsePayloadIntoAttributes(payload []byte, attr pcommon.Map) error {
	var log dnslog
	if err := gojson.Unmarshal(payload, &log); err != nil {
		return fmt.Errorf("failed to unmarshal DNS log: %w", err)
	}

	handleQueryAttributes(&log, attr)
	handleResponseAttributes(&log, attr)
	handleNetworkAttributes(&log, attr)
	handleTargetAttributes(&log, attr)
	handlePerformanceAndErrorAttributes(&log, attr)
	handleDNSFeatureAttributes(&log, attr)
	handleVMInstanceAttributes(&log, attr)

	return nil
}
