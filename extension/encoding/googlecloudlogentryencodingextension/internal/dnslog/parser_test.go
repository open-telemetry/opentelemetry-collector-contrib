// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestHandleQueryAttributes(t *testing.T) {
	tests := []struct {
		name     string
		log      *dnslog
		expected map[string]any
	}{
		{
			name: "all fields populated",
			log: &dnslog{
				QueryName: "example.com.",
				QueryType: "A",
			},
			expected: map[string]any{
				"dns.question.name": "example.com.",
				gcpDNSQueryType:     "A",
			},
		},
		{
			name: "empty fields",
			log: &dnslog{
				QueryName: "",
				QueryType: "",
			},
			expected: map[string]any{},
		},
		{
			name: "AAAA query type",
			log: &dnslog{
				QueryName: "ipv6.example.com.",
				QueryType: "AAAA",
			},
			expected: map[string]any{
				"dns.question.name": "ipv6.example.com.",
				gcpDNSQueryType:     "AAAA",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleQueryAttributes(tt.log, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleResponseAttributes(t *testing.T) {
	authAnswerFalse := false
	authAnswerTrue := true

	tests := []struct {
		name     string
		log      *dnslog
		expected map[string]any
	}{
		{
			name: "all fields populated",
			log: &dnslog{
				ResponseCode:           "NOERROR",
				AliasQueryResponseCode: "NOERROR",
				AuthAnswer:             &authAnswerFalse,
				Rdata:                  "example.com.\t300\tIN\ta\t1.2.3.4",
			},
			expected: map[string]any{
				gcpDNSResponseCode:           "NOERROR",
				gcpDNSAliasQueryResponseCode: "NOERROR",
				gcpDNSAuthAnswer:             false,
				gcpDNSAnswerData:             "example.com.\t300\tIN\ta\t1.2.3.4",
			},
		},
		{
			name: "authoritative answer true",
			log: &dnslog{
				ResponseCode: "NOERROR",
				AuthAnswer:   &authAnswerTrue,
				Rdata:        "test.com.\t300\tIN\ta\t5.6.7.8",
			},
			expected: map[string]any{
				gcpDNSResponseCode: "NOERROR",
				gcpDNSAuthAnswer:   true,
				gcpDNSAnswerData:   "test.com.\t300\tIN\ta\t5.6.7.8",
			},
		},
		{
			name: "nil auth answer",
			log: &dnslog{
				ResponseCode: "SERVFAIL",
				AuthAnswer:   nil,
			},
			expected: map[string]any{
				gcpDNSResponseCode: "SERVFAIL",
			},
		},
		{
			name: "empty fields",
			log: &dnslog{
				ResponseCode:           "",
				AliasQueryResponseCode: "",
				AuthAnswer:             nil,
				Rdata:                  "",
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleResponseAttributes(tt.log, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleNetworkAttributes(t *testing.T) {
	tests := []struct {
		name     string
		log      *dnslog
		expected map[string]any
	}{
		{
			name: "all fields populated",
			log: &dnslog{
				DestinationIP: "8.8.8.8",
				SourceNetwork: "vpc-network",
				SourceType:    "VM",
				SourceIP:      "10.0.0.1",
				Protocol:      "UDP",
				Location:      "us-central1",
			},
			expected: map[string]any{
				"server.address":       "8.8.8.8",
				gcpDNSClientVPCNetwork: "vpc-network",
				gcpDNSClientType:       "VM",
				"client.address":       "10.0.0.1",
				"network.transport":    "udp",
				"cloud.region":         "us-central1",
			},
		},
		{
			name: "TCP protocol",
			log: &dnslog{
				SourceIP: "192.168.1.1",
				Protocol: "TCP",
				Location: "europe-west1",
			},
			expected: map[string]any{
				"client.address":    "192.168.1.1",
				"network.transport": "tcp",
				"cloud.region":      "europe-west1",
			},
		},
		{
			name: "empty fields",
			log: &dnslog{
				DestinationIP: "",
				SourceNetwork: "",
				SourceType:    "",
				SourceIP:      "",
				Protocol:      "",
				Location:      "",
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleNetworkAttributes(tt.log, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleTargetAttributes(t *testing.T) {
	tests := []struct {
		name     string
		log      *dnslog
		expected map[string]any
	}{
		{
			name: "all fields populated",
			log: &dnslog{
				TargetName: "dns-target",
				TargetType: "forwarding-zone",
			},
			expected: map[string]any{
				gcpDNSServerName: "dns-target",
				gcpDNServerType:  "forwarding-zone",
			},
		},
		{
			name: "private zone target",
			log: &dnslog{
				TargetName: "private-zone-target",
				TargetType: "private-zone",
			},
			expected: map[string]any{
				gcpDNSServerName: "private-zone-target",
				gcpDNServerType:  "private-zone",
			},
		},
		{
			name: "empty fields",
			log: &dnslog{
				TargetName: "",
				TargetType: "",
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleTargetAttributes(tt.log, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandlePerformanceAndErrorAttributes(t *testing.T) {
	serverLatency35 := 3.5
	serverLatency0 := 0.0

	tests := []struct {
		name     string
		log      *dnslog
		expected map[string]any
	}{
		{
			name: "all fields populated",
			log: &dnslog{
				ServerLatency: &serverLatency35,
				EgressError:   "timeout",
				HealthyIps:    "10.0.0.1",
				UnhealthyIps:  "10.0.0.2",
			},
			expected: map[string]any{
				gcpDNSServerLatency: 3.5,
				gcpDNSEgressError:   "timeout",
				gcpDNSHealthyIPs:    "10.0.0.1",
				gcpDNSUnhealthyIPs:  "10.0.0.2",
			},
		},
		{
			name: "zero latency",
			log: &dnslog{
				ServerLatency: &serverLatency0,
				HealthyIps:    "10.0.0.1,10.0.0.2",
			},
			expected: map[string]any{
				gcpDNSServerLatency: 0.0,
				gcpDNSHealthyIPs:    "10.0.0.1,10.0.0.2",
			},
		},
		{
			name: "nil server latency",
			log: &dnslog{
				ServerLatency: nil,
				EgressError:   "connection refused",
			},
			expected: map[string]any{
				gcpDNSEgressError: "connection refused",
			},
		},
		{
			name: "empty fields",
			log: &dnslog{
				ServerLatency: nil,
				EgressError:   "",
				HealthyIps:    "",
				UnhealthyIps:  "",
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handlePerformanceAndErrorAttributes(tt.log, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleDNSFeatureAttributes(t *testing.T) {
	dns64True := true
	dns64False := false

	tests := []struct {
		name     string
		log      *dnslog
		expected map[string]any
	}{
		{
			name: "dns64 translated true",
			log: &dnslog{
				DNS64Translated: &dns64True,
			},
			expected: map[string]any{
				gcpDNSDNS64Translated: true,
			},
		},
		{
			name: "dns64 translated false",
			log: &dnslog{
				DNS64Translated: &dns64False,
			},
			expected: map[string]any{
				gcpDNSDNS64Translated: false,
			},
		},
		{
			name: "nil dns64 translated",
			log: &dnslog{
				DNS64Translated: nil,
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleDNSFeatureAttributes(tt.log, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleVMInstanceAttributes(t *testing.T) {
	vmInstanceID := int64(123456789)
	vmInstanceIDZero := int64(0)

	tests := []struct {
		name     string
		log      *dnslog
		expected map[string]any
	}{
		{
			name: "all fields populated",
			log: &dnslog{
				VMInstanceID:   &vmInstanceID,
				VMInstanceName: "test-instance",
				VMProjectID:    "test-project",
				VMZoneName:     "us-central1-a",
			},
			expected: map[string]any{
				"host.id":                 int64(123456789),
				"host.name":               "test-instance",
				gcpProjectID:              "test-project",
				"cloud.availability_zone": "us-central1-a",
			},
		},
		{
			name: "zero instance ID",
			log: &dnslog{
				VMInstanceID:   &vmInstanceIDZero,
				VMInstanceName: "instance-0",
			},
			expected: map[string]any{
				"host.id":   int64(0),
				"host.name": "instance-0",
			},
		},
		{
			name: "nil instance ID",
			log: &dnslog{
				VMInstanceID:   nil,
				VMInstanceName: "test-instance",
				VMProjectID:    "test-project",
			},
			expected: map[string]any{
				"host.name":  "test-instance",
				gcpProjectID: "test-project",
			},
		},
		{
			name: "empty fields",
			log: &dnslog{
				VMInstanceID:   nil,
				VMInstanceName: "",
				VMProjectID:    "",
				VMZoneName:     "",
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleVMInstanceAttributes(tt.log, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestParsePayloadIntoAttributes(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "complete DNS log with all fields",
			payload: []byte(`{
				"protocol": "UDP",
				"rdata": "logging.googleapis.com.\t300\tIN\ta\t173.194.76.95",
				"vmInstanceId": 2838092002611613700,
				"responseCode": "NOERROR",
				"vmInstanceIdString": "2838092002611613780",
				"vmZoneName": "europe-west1-c",
				"serverLatency": 5,
				"sourceNetwork": "default",
				"queryType": "A",
				"sourceIP": "10.132.0.43",
				"authAnswer": false,
				"vmProjectId": "elastic-logstash",
				"vmInstanceName": "228025391384.instance-20251126-211431",
				"dns64Translated": false,
				"queryName": "logging.googleapis.com."
			}`),
			expected: map[string]any{
				"dns.question.name":       "logging.googleapis.com.",
				gcpDNSQueryType:           "A",
				gcpDNSResponseCode:        "NOERROR",
				gcpDNSAuthAnswer:          false,
				gcpDNSAnswerData:          "logging.googleapis.com.\t300\tIN\ta\t173.194.76.95",
				gcpDNSClientVPCNetwork:    "default",
				"client.address":          "10.132.0.43",
				"network.transport":       "udp",
				gcpDNSServerLatency:       5.0,
				gcpDNSDNS64Translated:     false,
				"host.id":                 int64(2838092002611613700),
				"host.name":               "228025391384.instance-20251126-211431",
				gcpProjectID:              "elastic-logstash",
				"cloud.availability_zone": "europe-west1-c",
			},
		},
		{
			name: "DNS log with minimal fields",
			payload: []byte(`{
				"queryName": "example.com.",
				"queryType": "AAAA",
				"responseCode": "NXDOMAIN"
			}`),
			expected: map[string]any{
				"dns.question.name": "example.com.",
				gcpDNSQueryType:     "AAAA",
				gcpDNSResponseCode:  "NXDOMAIN",
			},
		},
		{
			name: "DNS log with target and network attributes",
			payload: []byte(`{
				"queryName": "test.example.com.",
				"queryType": "A",
				"responseCode": "NOERROR",
				"sourceIP": "192.168.1.100",
				"destinationIP": "8.8.8.8",
				"sourceNetwork": "vpc-network",
				"source_type": "VM",
				"target_name": "dns-target",
				"target_type": "forwarding-zone",
				"location": "us-central1"
			}`),
			expected: map[string]any{
				"dns.question.name":    "test.example.com.",
				gcpDNSQueryType:        "A",
				gcpDNSResponseCode:     "NOERROR",
				"client.address":       "192.168.1.100",
				"server.address":       "8.8.8.8",
				gcpDNSClientVPCNetwork: "vpc-network",
				gcpDNSClientType:       "VM",
				gcpDNSServerName:       "dns-target",
				gcpDNServerType:        "forwarding-zone",
				"cloud.region":         "us-central1",
			},
		},
		{
			name: "DNS log with error and health check attributes",
			payload: []byte(`{
				"queryName": "error.example.com.",
				"queryType": "A",
				"responseCode": "SERVFAIL",
				"serverLatency": 2.5,
				"egressError": "connection timeout",
				"healthyIps": "10.0.0.1,10.0.0.2",
				"unhealthyIps": "10.0.0.3"
			}`),
			expected: map[string]any{
				"dns.question.name": "error.example.com.",
				gcpDNSQueryType:     "A",
				gcpDNSResponseCode:  "SERVFAIL",
				gcpDNSServerLatency: 2.5,
				gcpDNSEgressError:   "connection timeout",
				gcpDNSHealthyIPs:    "10.0.0.1,10.0.0.2",
				gcpDNSUnhealthyIPs:  "10.0.0.3",
			},
		},
		{
			name: "DNS log with alias query response",
			payload: []byte(`{
				"queryName": "alias.example.com.",
				"queryType": "CNAME",
				"responseCode": "NOERROR",
				"alias_query_response_code": "NOERROR",
				"rdata": "alias.example.com.\t300\tIN\tcname\ttarget.example.com."
			}`),
			expected: map[string]any{
				"dns.question.name":          "alias.example.com.",
				gcpDNSQueryType:              "CNAME",
				gcpDNSResponseCode:           "NOERROR",
				gcpDNSAliasQueryResponseCode: "NOERROR",
				gcpDNSAnswerData:             "alias.example.com.\t300\tIN\tcname\ttarget.example.com.",
			},
		},
		{
			name: "DNS log with DNS64 translation",
			payload: []byte(`{
				"queryName": "ipv6.example.com.",
				"queryType": "AAAA",
				"responseCode": "NOERROR",
				"dns64Translated": true,
				"protocol": "TCP"
			}`),
			expected: map[string]any{
				"dns.question.name":   "ipv6.example.com.",
				gcpDNSQueryType:       "AAAA",
				gcpDNSResponseCode:    "NOERROR",
				gcpDNSDNS64Translated: true,
				"network.transport":   "tcp",
			},
		},
		{
			name:    "invalid JSON payload",
			payload: []byte(`{invalid json`),
			wantErr: true,
		},
		{
			name:     "empty payload",
			payload:  []byte(`{}`),
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			err := ParsePayloadIntoAttributes(tt.payload, attr)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, len(tt.expected), attr.Len(), "attribute count mismatch")

			for key, expectedValue := range tt.expected {
				val, exists := attr.Get(key)
				assert.True(t, exists, "expected attribute %q to exist", key)

				switch v := expectedValue.(type) {
				case string:
					assert.Equal(t, v, val.Str(), "mismatch for key %q", key)
				case int64:
					assert.Equal(t, v, val.Int(), "mismatch for key %q", key)
				case float64:
					assert.Equal(t, v, val.Double(), "mismatch for key %q", key)
				case bool:
					assert.Equal(t, v, val.Bool(), "mismatch for key %q", key)
				}
			}
		})
	}
}
