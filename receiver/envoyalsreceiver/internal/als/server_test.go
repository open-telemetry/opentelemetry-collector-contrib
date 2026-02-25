// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package als // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver/internal/als"

import (
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	alsdata "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	alsv3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestToLogs(t *testing.T) {
	tm, err := time.Parse(time.RFC3339Nano, "2020-07-30T01:01:01.123456789Z")
	require.NoError(t, err)
	ts := int64(pcommon.NewTimestampFromTime(tm))

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

	nodeID := &corev3.Node{
		Id: "node-id",
	}

	cases := []struct {
		name  string
		input *alsv3.StreamAccessLogsMessage
		want  plog.Logs
	}{
		{
			name: "tcp",
			input: &alsv3.StreamAccessLogsMessage{
				Identifier: &alsv3.StreamAccessLogsMessage_Identifier{
					Node: &corev3.Node{
						Id: "node-id",
					},
					LogName: "test-log-name",
				},
				LogEntries: &alsv3.StreamAccessLogsMessage_TcpLogs{
					TcpLogs: &alsv3.StreamAccessLogsMessage_TCPAccessLogEntries{
						LogEntry: []*alsdata.TCPAccessLogEntry{
							{
								CommonProperties: &alsdata.AccessLogCommon{
									StartTime: timestamppb.New(tm),
								},
								ConnectionProperties: &alsdata.ConnectionProperties{
									ReceivedBytes: 10,
									SentBytes:     20,
								},
							},
						},
					},
				},
			},
			want: generateLogs(map[string]string{
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
		{
			name: "http",
			input: &alsv3.StreamAccessLogsMessage{
				Identifier: &alsv3.StreamAccessLogsMessage_Identifier{
					Node: &corev3.Node{
						Id: "node-id",
					},
					LogName: "test-log-name",
				},
				LogEntries: &alsv3.StreamAccessLogsMessage_HttpLogs{
					HttpLogs: &alsv3.StreamAccessLogsMessage_HTTPAccessLogEntries{
						LogEntry: []*alsdata.HTTPAccessLogEntry{
							{
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
							},
						},
					},
				},
			},
			want: generateLogs(map[string]string{
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toLogs(tc.input)

			err := plogtest.CompareLogs(tc.want, got, plogtest.IgnoreObservedTimestamp())
			require.NoError(t, err)
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
