// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxynlb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
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
				string(semconv.SourceAddressKey):       "68.168.189.182",
				string(semconv.SourcePortKey):          int64(52900),
				string(semconv.DestinationAddressKey):  "35.209.164.189",
				string(semconv.DestinationPortKey):     int64(80),
				string(semconv.NetworkProtocolNameKey): "tcp",
			},
		},
		"nil connection": {
			conn:         nil,
			expectedAttr: map[string]any{},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleConnection(tt.conn, attr)
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
		gcpProxyNLBConnectionStartTime: start.Format(time.RFC3339Nano),
		gcpProxyNLBConnectionEndTime:   end.Format(time.RFC3339Nano),
	}, attr.AsRaw())
}

func TestParsePayloadIntoAttributes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		payload    []byte
		assertFunc func(t *testing.T, attr pcommon.Map, err error)
	}{
		"invalid json": {
			payload: []byte("not-json"),
			assertFunc: func(t *testing.T, _ pcommon.Map, err error) {
				require.ErrorContains(t, err, "failed to unmarshal Proxy NLB log payload")
			},
		},
		"unexpected type": {
			payload: []byte(`{"@type":"invalid"}`),
			assertFunc: func(t *testing.T, _ pcommon.Map, err error) {
				require.ErrorContains(t, err, "unexpected log type")
			},
		},
		"invalid bytes": {
			payload: []byte(`{"serverBytesReceived":"abc"}`),
			assertFunc: func(t *testing.T, _ pcommon.Map, err error) {
				require.ErrorContains(t, err, "failed to add server bytes received")
			},
		},
		"success": {
			payload: []byte(`{
				"@type":"type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"connection":{
					"clientIp":"68.168.189.182",
					"clientPort":52900,
					"protocol":6,
					"serverIp":"35.209.164.189",
					"serverPort":80
				},
				"startTime":"2025-11-17T22:21:57.480419Z",
				"endTime":"2025-11-17T22:21:57.500505Z",
				"serverBytesReceived":"83",
				"serverBytesSent":"853"
			}`),
			assertFunc: func(t *testing.T, attr pcommon.Map, err error) {
				require.NoError(t, err)
				require.Equal(t, map[string]any{
					string(semconv.SourceAddressKey):       "68.168.189.182",
					string(semconv.SourcePortKey):          int64(52900),
					string(semconv.DestinationAddressKey):  "35.209.164.189",
					string(semconv.DestinationPortKey):     int64(80),
					string(semconv.NetworkProtocolNameKey): "tcp",
					gcpProxyNLBConnectionStartTime:         "2025-11-17T22:21:57.480419Z",
					gcpProxyNLBConnectionEndTime:           "2025-11-17T22:21:57.500505Z",
					gcpProxyNLBServerBytesReceived:         int64(83),
					gcpProxyNLBServerBytesSent:             int64(853),
				}, attr.AsRaw())
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			err := ParsePayloadIntoAttributes(tt.payload, attr)
			tt.assertFunc(t, attr, err)
		})
	}
}
