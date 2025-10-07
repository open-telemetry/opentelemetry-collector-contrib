// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// int64Ptr returns a pointer to the given int64 value
func int64Ptr(v int64) *int64 {
	return &v
}

func TestHandleConnection(t *testing.T) {
	tests := map[string]struct {
		connection   *connection
		expectedAttr map[string]any
	}{
		"tcp connection": {
			connection: &connection{
				Protocol: int64Ptr(6),
				SrcIP:    "10.128.0.41",
				DestIP:   "35.191.240.41",
				SrcPort:  int64Ptr(443),
				DestPort: int64Ptr(40708),
			},
			expectedAttr: map[string]any{
				string(semconv.NetworkProtocolNameKey): "tcp",
				string(semconv.SourceAddressKey):       "10.128.0.41",
				string(semconv.DestinationAddressKey):  "35.191.240.41",
				string(semconv.SourcePortKey):          int64(443),
				string(semconv.DestinationPortKey):     int64(40708),
			},
		},
		"icmp connection": {
			connection: &connection{
				Protocol: int64Ptr(1),
				SrcIP:    "54.237.57.118",
				DestIP:   "10.160.0.36",
			},
			expectedAttr: map[string]any{
				string(semconv.NetworkProtocolNameKey): "icmp",
				string(semconv.SourceAddressKey):       "54.237.57.118",
				string(semconv.DestinationAddressKey):  "10.160.0.36",
			},
		},
		"udp connection": {
			connection: &connection{
				Protocol: int64Ptr(17),
				SrcIP:    "192.168.1.1",
				DestIP:   "192.168.1.2",
				SrcPort:  int64Ptr(53),
				DestPort: int64Ptr(53),
			},
			expectedAttr: map[string]any{
				string(semconv.NetworkProtocolNameKey): "udp",
				string(semconv.SourceAddressKey):       "192.168.1.1",
				string(semconv.DestinationAddressKey):  "192.168.1.2",
				string(semconv.SourcePortKey):          int64(53),
				string(semconv.DestinationPortKey):     int64(53),
			},
		},
		"unknown protocol": {
			connection: &connection{
				Protocol: int64Ptr(250),
				SrcIP:    "10.0.0.1",
				DestIP:   "10.0.0.2",
			},
			expectedAttr: map[string]any{
				// 250 is not present in the protocolNames map,
				// so we don't expect it to be in the attributes
				string(semconv.SourceAddressKey):      "10.0.0.1",
				string(semconv.DestinationAddressKey): "10.0.0.2",
			},
		},
		"nil connection": {
			connection:   nil,
			expectedAttr: map[string]any{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleConnection(tt.connection, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleNetworkService(t *testing.T) {
	tests := map[string]struct {
		networkService *networkService
		expectedAttr   map[string]any
	}{
		"with dscp": {
			networkService: &networkService{
				DSCP: int64Ptr(32),
			},
			expectedAttr: map[string]any{
				gcpVPCFlowNetworkServiceDSCP: int64(32),
			},
		},
		"nil network service": {
			networkService: nil,
			expectedAttr:   map[string]any{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleNetworkService(tt.networkService, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleInstance(t *testing.T) {
	tests := map[string]struct {
		instance     *instance
		side         flowSide
		expectedAttr map[string]any
	}{
		"source instance with mig": {
			instance: &instance{
				ProjectID: "elastic-obs-integrations-dev",
				Region:    "us-central1",
				VMName:    "proxy-crest-new-group-b51k",
				Zone:      "us-central1-a",
				ManagedInstanceGroup: &managedInstanceGroup{
					Name: "proxy-crest-new-group",
					Zone: "us-central1-a",
				},
			},
			side: src,
			expectedAttr: map[string]any{
				gcpVPCFlowSourceInstanceProjectID: "elastic-obs-integrations-dev",
				gcpVPCFlowSourceInstanceVMRegion:  "us-central1",
				gcpVPCFlowSourceInstanceVMName:    "proxy-crest-new-group-b51k",
				gcpVPCFlowSourceInstanceVMZone:    "us-central1-a",
				gcpVPCFlowSourceInstanceMIGName:   "proxy-crest-new-group",
				gcpVPCFlowSourceInstanceMIGZone:   "us-central1-a",
			},
		},
		"destination instance without mig": {
			instance: &instance{
				ProjectID: "elastic-obs-integrations-dev",
				Region:    "asia-south1",
				VMName:    "service-integration-dev-idc-ubuntu25-4",
				Zone:      "asia-south1-c",
			},
			side: dest,
			expectedAttr: map[string]any{
				gcpVPCFlowDestInstanceProjectID: "elastic-obs-integrations-dev",
				gcpVPCFlowDestInstanceVMRegion:  "asia-south1",
				gcpVPCFlowDestInstanceVMName:    "service-integration-dev-idc-ubuntu25-4",
				gcpVPCFlowDestInstanceVMZone:    "asia-south1-c",
			},
		},
		"nil instance": {
			instance:     nil,
			side:         src,
			expectedAttr: map[string]any{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleInstance(tt.instance, tt.side, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleLocation(t *testing.T) {
	tests := map[string]struct {
		location     *location
		side         flowSide
		expectedAttr map[string]any
	}{
		"source location": {
			location: &location{
				ASN:       int64Ptr(14618),
				City:      "Ashburn",
				Continent: "America",
				Country:   "usa",
				Region:    "Virginia",
			},
			side: src,
			expectedAttr: map[string]any{
				gcpVPCFlowSourceASN:          int64(14618),
				gcpVPCFlowSourceGeoCity:      "Ashburn",
				gcpVPCFlowSourceGeoContinent: "America",
				gcpVPCFlowSourceGeoCountry:   "usa",
				gcpVPCFlowSourceGeoRegion:    "Virginia",
			},
		},
		"destination location": {
			location: &location{
				ASN:       int64Ptr(137718),
				Continent: "Asia",
				Country:   "chn",
			},
			side: dest,
			expectedAttr: map[string]any{
				gcpVPCFlowDestASN:          int64(137718),
				gcpVPCFlowDestGeoContinent: "Asia",
				gcpVPCFlowDestGeoCountry:   "chn",
			},
		},
		"nil location": {
			location:     nil,
			side:         src,
			expectedAttr: map[string]any{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleLocation(tt.location, tt.side, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleVPC(t *testing.T) {
	tests := map[string]struct {
		vpc          *vpc
		side         flowSide
		expectedAttr map[string]any
	}{
		"source vpc": {
			vpc: &vpc{
				ProjectID:        "elastic-obs-integrations-dev",
				SubnetworkName:   "default",
				SubnetworkRegion: "us-central1",
				VPCName:          "default",
			},
			side: src,
			expectedAttr: map[string]any{
				gcpVPCFlowSourceProjectID:    "elastic-obs-integrations-dev",
				gcpVPCFlowSourceSubnetName:   "default",
				gcpVPCFlowSourceSubnetRegion: "us-central1",
				gcpVPCFlowSourceVPCName:      "default",
			},
		},
		"destination vpc": {
			vpc: &vpc{
				ProjectID:        "elastic-obs-integrations-dev",
				SubnetworkName:   "default",
				SubnetworkRegion: "asia-south1",
				VPCName:          "default",
			},
			side: dest,
			expectedAttr: map[string]any{
				gcpVPCFlowDestProjectID:    "elastic-obs-integrations-dev",
				gcpVPCFlowDestSubnetName:   "default",
				gcpVPCFlowDestSubnetRegion: "asia-south1",
				gcpVPCFlowDestVPCName:      "default",
			},
		},
		"nil vpc": {
			vpc:          nil,
			side:         src,
			expectedAttr: map[string]any{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleVPC(tt.vpc, tt.side, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleInternetRoutingDetails(t *testing.T) {
	tests := map[string]struct {
		ird          *internetRoutingDetails
		expectedAttr map[string]any
	}{
		"with as paths": {
			ird: &internetRoutingDetails{
				EgressASPath: []egressASPath{
					{
						ASDetails: []asDetails{
							{ASN: int64Ptr(58453)},
							{ASN: int64Ptr(9808)},
							{ASN: int64Ptr(38019)},
							{ASN: int64Ptr(137718)},
						},
					},
				},
			},
			expectedAttr: map[string]any{
				gcpVPCFlowEgressASPaths: []any{
					map[string]any{
						"as_details": []any{
							map[string]any{"asn": int64(58453)},
							map[string]any{"asn": int64(9808)},
							map[string]any{"asn": int64(38019)},
							map[string]any{"asn": int64(137718)},
						},
					},
				},
			},
		},
		"nil internet routing details": {
			ird:          nil,
			expectedAttr: map[string]any{},
		},
		"empty egress as path": {
			ird: &internetRoutingDetails{
				EgressASPath: []egressASPath{},
			},
			expectedAttr: map[string]any{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleInternetRoutingDetails(tt.ird, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestParsePayloadIntoAttributes(t *testing.T) {
	tests := map[string]struct {
		payload      []byte
		expectedAttr map[string]any
		expectsErr   string
	}{
		"invalid payload": {
			payload:    []byte("invalid"),
			expectsErr: "failed to unmarshal VPC flow log payload",
		},
		"empty payload": {
			payload:      []byte(`{}`),
			expectedAttr: map[string]any{},
		},
		"invalid bytes_sent": {
			payload: []byte(`{
				"bytes_sent": "invalid"
			}`),
			expectsErr: "failed to add bytes sent",
		},
		"invalid packets_sent": {
			payload: []byte(`{
				"packets_sent": "invalid"
			}`),
			expectsErr: "failed to add packets sent",
		},
		"small payload": {
			payload: []byte(`{
				"reporter": "SRC",
				"bytes_sent": "0",
				"packets_sent": "640",
				"start_time": "2025-09-27T21:12:02.937646004Z",
				"end_time": "2025-09-27T21:13:02.927646004Z"
			}`),
			expectedAttr: map[string]any{
				gcpVPCFlowReporter:    "SRC",
				gcpVPCFlowBytesSent:   int64(0),
				gcpVPCFlowPacketsSent: int64(640),
				gcpVPCFlowStartTime:   int64(1759007522937646004),
				gcpVPCFlowEndTime:     int64(1759007582927646004),
			},
		},
		"complete payload": {
			payload: []byte(`{
				"connection": {
					"protocol": 6,
					"src_ip": "10.128.0.41",
					"dest_ip": "35.191.240.41",
					"src_port": 443,
					"dest_port": 40708
				},
				"reporter": "SRC",
				"bytes_sent": "0",
				"packets_sent": "640",
				"start_time": "2025-09-27T21:12:02.937646004Z",
				"end_time": "2025-09-27T21:15:03.837646004Z",
				"network_service": {
					"dscp": 32
				},
				"src_instance": {
					"project_id": "elastic-obs-integrations-dev",
					"region": "us-central1",
					"vm_name": "proxy-crest-new-group-b51k",
					"zone": "us-central1-a",
					"managed_instance_group": {
						"name": "proxy-crest-new-group",
						"zone": "us-central1-a"
					}
				},
				"src_vpc": {
					"project_id": "elastic-obs-integrations-dev",
					"subnetwork_name": "default",
					"subnetwork_region": "us-central1",
					"vpc_name": "default"
				}
			}`),
			expectedAttr: map[string]any{
				string(semconv.NetworkProtocolNameKey): "tcp",
				string(semconv.SourceAddressKey):       "10.128.0.41",
				string(semconv.DestinationAddressKey):  "35.191.240.41",
				string(semconv.SourcePortKey):          int64(443),
				string(semconv.DestinationPortKey):     int64(40708),
				gcpVPCFlowReporter:                     "SRC",
				gcpVPCFlowBytesSent:                    int64(0),
				gcpVPCFlowPacketsSent:                  int64(640),
				gcpVPCFlowStartTime:                    int64(1759007522937646004),
				gcpVPCFlowEndTime:                      int64(1759007703837646004),
				gcpVPCFlowNetworkServiceDSCP:           int64(32),
				gcpVPCFlowSourceInstanceProjectID:      "elastic-obs-integrations-dev",
				gcpVPCFlowSourceInstanceVMRegion:       "us-central1",
				gcpVPCFlowSourceInstanceVMName:         "proxy-crest-new-group-b51k",
				gcpVPCFlowSourceInstanceVMZone:         "us-central1-a",
				gcpVPCFlowSourceInstanceMIGName:        "proxy-crest-new-group",
				gcpVPCFlowSourceInstanceMIGZone:        "us-central1-a",
				gcpVPCFlowSourceProjectID:              "elastic-obs-integrations-dev",
				gcpVPCFlowSourceSubnetName:             "default",
				gcpVPCFlowSourceSubnetRegion:           "us-central1",
				gcpVPCFlowSourceVPCName:                "default",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			err := ParsePayloadIntoAttributes(tt.payload, attr)
			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedAttr, attr.AsRaw())
			}
		})
	}
}
