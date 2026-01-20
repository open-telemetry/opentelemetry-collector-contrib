// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package passthroughnlb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestHandleConnection(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		conn         *connection
		expectedAttr map[string]any
	}{
		"tcp connection": {
			conn: &connection{
				ClientIP:   "68.168.189.182",
				ClientPort: int64Ptr(52900),
				Protocol:   int64Ptr(6),
				ServerIP:   "35.209.164.189",
				ServerPort: int64Ptr(80),
			},
			expectedAttr: map[string]any{
				"client.address":    "68.168.189.182",
				"client.port":       int64(52900),
				"server.address":    "35.209.164.189",
				"server.port":       int64(80),
				"network.transport": "tcp",
			},
		},
		"nil connection": {
			conn:         nil,
			expectedAttr: map[string]any{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			err := handleConnection(tt.conn, attr)
			require.NoError(t, err)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleConnectionServerAddressAlreadySet(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		conn         *connection
		expectedErr  error
		expectedAttr map[string]any
	}{
		"tcp connection": {
			conn: &connection{
				ClientIP:   "68.168.189.182",
				ClientPort: int64Ptr(52900),
				Protocol:   int64Ptr(6),
				ServerIP:   "35.209.164.189",
				ServerPort: int64Ptr(80),
			},
			expectedErr: errServerAddress,
			expectedAttr: map[string]any{
				"client.address":    "68.168.189.182",
				"client.port":       int64(52900),
				"server.address":    "10.0.0.1",
				"server.port":       int64(80),
				"network.transport": "tcp",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			attr.PutStr("server.address", "10.0.0.1")
			err := handleConnection(tt.conn, attr)
			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleTimestamps(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, 11, 17, 22, 21, 57, 480419000, time.UTC)
	end := time.Date(2025, 11, 17, 22, 21, 57, 500505000, time.UTC)

	attr := pcommon.NewMap()
	handleTimestamps(timePtr(start), timePtr(end), attr)

	require.Equal(t, map[string]any{
		gcpPassthroughNLBPacketsStartTime: start.Format(time.RFC3339Nano),
		gcpPassthroughNLBPacketsEndTime:   end.Format(time.RFC3339Nano),
	}, attr.AsRaw())
}

func TestParsePayloadIntoAttributes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		payload      []byte
		expectedAttr map[string]any
		expectedErr  error
	}{
		"invalid json": {
			payload:     []byte("not-json"),
			expectedErr: errUnmarshalPayload,
		},
		"unexpected type": {
			payload:     []byte(`{"@type":"invalid"}`),
			expectedErr: errUnexpectedLogType,
		},
		"invalid bytes": {
			payload:     []byte(`{"bytesReceived":"abc"}`),
			expectedErr: errUnexpectedLogType,
		},
		"success - external NLB": {
			payload: []byte(`{
				"@type":"type.googleapis.com/google.cloud.loadbalancing.type.ExternalNetworkLoadBalancerLogEntry",
				"connection":{
					"clientIp":"68.168.189.182",
					"clientPort":52900,
					"protocol":6,
					"serverIp":"35.209.164.189",
					"serverPort":80
				},
				"startTime":"2025-11-17T22:21:57.480419Z",
				"endTime":"2025-11-17T22:21:57.500505Z",
				"bytesReceived":"83",
				"bytesSent":"853",
				"packetsReceived":"12",
				"packetsSent":"12",
				"rtt":"0.063198476s"
			}`),
			expectedAttr: map[string]any{
				"client.address":                  "68.168.189.182",
				"client.port":                     int64(52900),
				"server.address":                  "35.209.164.189",
				"server.port":                     int64(80),
				"network.transport":               "tcp",
				gcpPassthroughNLBPacketsStartTime: "2025-11-17T22:21:57.480419Z",
				gcpPassthroughNLBPacketsEndTime:   "2025-11-17T22:21:57.500505Z",
				gcpPassthroughNLBBytesReceived:    int64(83),
				gcpPassthroughNLBBytesSent:        int64(853),
				gcpPassthroughNLBPacketsReceived:  int64(12),
				gcpPassthroughNLBPacketsSent:      int64(12),
				gcpPassthroughNLBRTT:              float64(0.063198476),
			},
		},
		"success - internal NLB": {
			payload: []byte(`{
				"@type":"type.googleapis.com/google.cloud.loadbalancing.type.InternalNetworkLoadBalancerLogEntry",
				"connection":{
					"clientIp":"10.0.0.5",
					"clientPort":45678,
					"protocol":6,
					"serverIp":"10.0.0.10",
					"serverPort":443
				},
				"startTime":"2025-11-17T22:21:57.480419Z",
				"endTime":"2025-11-17T22:21:57.500505Z",
				"bytesReceived":"100",
				"bytesSent":"500",
				"packetsReceived":"5",
				"packetsSent":"8",
				"rtt":"0.005s"
			}`),
			expectedAttr: map[string]any{
				"client.address":                  "10.0.0.5",
				"client.port":                     int64(45678),
				"server.address":                  "10.0.0.10",
				"server.port":                     int64(443),
				"network.transport":               "tcp",
				gcpPassthroughNLBPacketsStartTime: "2025-11-17T22:21:57.480419Z",
				gcpPassthroughNLBPacketsEndTime:   "2025-11-17T22:21:57.500505Z",
				gcpPassthroughNLBBytesReceived:    int64(100),
				gcpPassthroughNLBBytesSent:        int64(500),
				gcpPassthroughNLBPacketsReceived:  int64(5),
				gcpPassthroughNLBPacketsSent:      int64(8),
				gcpPassthroughNLBRTT:              float64(0.005),
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			err := ParsePayloadIntoAttributes(tt.payload, attr)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedAttr, attr.AsRaw())
			}
		})
	}
}
