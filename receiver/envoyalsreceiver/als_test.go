// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package envoyalsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver"

import (
	"context"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	alsdata "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	alsv3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver/internal/metadata"
)

func startGRPCServer(t *testing.T) (*grpc.ClientConn, *consumertest.LogsSink) {
	config := &Config{
		ServerConfig: configgrpc.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  testutil.GetAvailableLocalAddress(t),
				Transport: confignet.TransportTypeTCP,
			},
		},
	}
	sink := new(consumertest.LogsSink)

	set := receivertest.NewNopSettings(metadata.Type)
	lr, err := newALSReceiver(config, sink, set)
	require.NoError(t, err)

	require.NoError(t, lr.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, lr.Shutdown(context.Background())) })

	conn, err := grpc.NewClient(config.NetAddr.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return conn, sink
}

func TestLogs(t *testing.T) {
	// Start grpc server
	conn, sink := startGRPCServer(t)
	defer func() {
		_ = conn.Close()
	}()

	client, err := alsv3.NewAccessLogServiceClient(conn).StreamAccessLogs(context.Background())
	require.NoError(t, err)

	tm, err := time.Parse(time.RFC3339Nano, "2020-07-30T01:01:01.123456789Z")
	require.NoError(t, err)
	ts := int64(pcommon.NewTimestampFromTime(tm))

	nodeID := &corev3.Node{
		Id:      "test-id",
		Cluster: "test-cluster",
	}

	identifier := &alsv3.StreamAccessLogsMessage_Identifier{
		Node:    nodeID,
		LogName: "test-log-name",
	}

	httpLog := &alsdata.HTTPAccessLogEntry{
		CommonProperties: &alsdata.AccessLogCommon{
			StartTime: timestamppb.New(tm),
		},
		Request: &alsdata.HTTPRequestProperties{
			Path:      "/test",
			Authority: "example.com",
		},
		Response: &alsdata.HTTPResponseProperties{
			ResponseCode: wrapperspb.UInt32(200),
		},
	}

	tcpLog := &alsdata.TCPAccessLogEntry{
		CommonProperties: &alsdata.AccessLogCommon{
			StartTime: timestamppb.New(tm),
		},
		ConnectionProperties: &alsdata.ConnectionProperties{
			ReceivedBytes: 10,
			SentBytes:     20,
		},
	}

	tests := []struct {
		name     string
		message  *alsv3.StreamAccessLogsMessage
		expected plog.Logs
	}{
		{
			name: "http",
			message: &alsv3.StreamAccessLogsMessage{
				Identifier: identifier,
				LogEntries: &alsv3.StreamAccessLogsMessage_HttpLogs{
					HttpLogs: &alsv3.StreamAccessLogsMessage_HTTPAccessLogEntries{
						LogEntry: []*alsdata.HTTPAccessLogEntry{
							httpLog,
						},
					},
				},
			},
			expected: generateLogs(map[string]string{
				"node":     nodeID.String(),
				"log_name": "test-log-name",
			}, []Log{
				{
					Timestamp: ts,
					Attributes: map[string]any{
						"api_version": "v3",
						"log_type":    "http",
					},
					Body: httpLog.String(),
				},
			}),
		},
		{
			name: "tcp",
			message: &alsv3.StreamAccessLogsMessage{
				Identifier: identifier,
				LogEntries: &alsv3.StreamAccessLogsMessage_TcpLogs{
					TcpLogs: &alsv3.StreamAccessLogsMessage_TCPAccessLogEntries{
						LogEntry: []*alsdata.TCPAccessLogEntry{
							tcpLog,
						},
					},
				},
			},
			expected: generateLogs(map[string]string{
				"node":     nodeID.String(),
				"log_name": "test-log-name",
			}, []Log{
				{
					Timestamp: ts,
					Attributes: map[string]any{
						"api_version": "v3",
						"log_type":    "tcp",
					},
					Body: tcpLog.String(),
				},
			}),
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = client.Send(tt.message)
			require.NoError(t, err, "should not have failed to post logs")

			require.Eventually(t, func() bool {
				gotLogs := sink.AllLogs()

				err := plogtest.CompareLogs(tt.expected, gotLogs[i], plogtest.IgnoreObservedTimestamp())
				if err == nil {
					return true
				}
				t.Logf("Logs not received yet: %v", err)
				return false
			}, 5*time.Second, 100*time.Millisecond)
		})
	}
}

type Log struct {
	Timestamp  int64
	Body       string
	Attributes map[string]any
}

func generateLogs(resourceAttrs map[string]string, logs []Log) plog.Logs {
	ld := plog.NewLogs()
	rls := ld.ResourceLogs().AppendEmpty()
	for k, v := range resourceAttrs {
		rls.Resource().Attributes().PutStr(k, v)
	}
	logSlice := rls.ScopeLogs().AppendEmpty().LogRecords()

	for _, log := range logs {
		lr := logSlice.AppendEmpty()
		_ = lr.Attributes().FromRaw(log.Attributes)
		lr.SetTimestamp(pcommon.Timestamp(log.Timestamp))
		lr.Body().SetStr(log.Body)
	}
	return ld
}
