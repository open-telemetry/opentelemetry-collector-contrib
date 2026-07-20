// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

func TestGrpcService_ProcessTelemetryData_ContextBagAttributes(t *testing.T) {
	tests := []struct {
		name               string
		telemetry          *pb.Telemetry
		expectedDatapoints map[string]map[string]string
	}{
		{
			name: "copies context bag values onto step and numeric metrics",
			telemetry: &pb.Telemetry{
				NodeId:       &pb.Telemetry_NodeIdStr{NodeIdStr: "test-node-1"},
				EncodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
				MsgTimestamp: uint64(time.Date(2026, time.May, 28, 12, 0, 0, 0, time.UTC).UnixMilli()),
				DataGpbkv: []*pb.TelemetryField{
					{
						Name: "interface",
						Fields: []*pb.TelemetryField{
							{
								Name: "keys",
								Fields: []*pb.TelemetryField{
									{
										Name: "interface-name",
										ValueByType: &pb.TelemetryField_StringValue{
											StringValue: "GigabitEthernet0/0/1",
										},
									},
									{
										Name: "name",
										ValueByType: &pb.TelemetryField_StringValue{
											StringValue: "GigabitEthernet0/0/1",
										},
									},
								},
							},
							{
								Name: "content",
								Fields: []*pb.TelemetryField{
									{
										Name: "admin-status",
										ValueByType: &pb.TelemetryField_StringValue{
											StringValue: "up",
										},
									},
									{
										Name: "rx-pkts",
										ValueByType: &pb.TelemetryField_Uint64Value{
											Uint64Value: 1234567,
										},
									},
								},
							},
						},
					},
				},
			},
			expectedDatapoints: map[string]map[string]string{
				"cisco.interface.keys.interface-name_info": {
					"node_id":        "test-node-1",
					"interface-name": "GigabitEthernet0/0/1",
					"interface":      "GigabitEthernet0/0/1",
					"name":           "GigabitEthernet0/0/1",
					"admin-status":   "up",
				},
				"cisco.interface.keys.name_info": {
					"node_id":        "test-node-1",
					"interface-name": "GigabitEthernet0/0/1",
					"interface":      "GigabitEthernet0/0/1",
					"name":           "GigabitEthernet0/0/1",
					"admin-status":   "up",
				},
				"cisco.interface.content.admin-status_info": {
					"node_id":        "test-node-1",
					"interface-name": "GigabitEthernet0/0/1",
					"interface":      "GigabitEthernet0/0/1",
					"name":           "GigabitEthernet0/0/1",
					"admin-status":   "up",
					"value":          "up",
				},
				"cisco.interface.content.rx-pkts": {
					"node_id":        "test-node-1",
					"interface-name": "GigabitEthernet0/0/1",
					"interface":      "GigabitEthernet0/0/1",
					"name":           "GigabitEthernet0/0/1",
					"admin-status":   "up",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidTestConfig()
			consumer := &consumertest.MetricsSink{}
			settings := createTestSettings()
			receiver := createMetricsReceiver(t.Context(), settings, config, consumer)

			yangParser := internal.NewYANGParser()
			yangParser.LoadBuiltinModules()
			service := &grpcService{
				receiver:   receiver.(*yangReceiver),
				yangParser: yangParser,
			}

			data, err := proto.Marshal(tt.telemetry)
			require.NoError(t, err)

			req := &pb.MdtDialoutArgs{ReqId: 1, Data: data}
			require.NoError(t, service.processTelemetryData(req))

			emitted := consumer.AllMetrics()
			require.Len(t, emitted, 1)

			resourceMetrics := emitted[0].ResourceMetrics()
			require.Equal(t, 1, resourceMetrics.Len())

			scopeMetrics := resourceMetrics.At(0).ScopeMetrics()
			require.Equal(t, 1, scopeMetrics.Len())

			metrics := scopeMetrics.At(0).Metrics()
			require.Equal(t, len(tt.expectedDatapoints), metrics.Len())

			seen := make(map[string]struct{}, metrics.Len())
			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				expectedAttrs, ok := tt.expectedDatapoints[metric.Name()]
				require.Truef(t, ok, "unexpected metric emitted: %s", metric.Name())

				seen[metric.Name()] = struct{}{}
				assertMetricDatapointsHaveAttributes(t, metric, expectedAttrs)
			}

			require.Len(t, seen, len(tt.expectedDatapoints))
		})
	}
}

func assertMetricDatapointsHaveAttributes(t *testing.T, metric pmetric.Metric, expectedAttrs map[string]string) {
	t.Helper()

	var datapoints pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		datapoints = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		datapoints = metric.Sum().DataPoints()
	default:
		require.Failf(t, "unexpected metric type", "metric %q has type %v", metric.Name(), metric.Type())
	}

	require.Positive(t, datapoints.Len(), "expected datapoints for metric %q", metric.Name())
	for i := 0; i < datapoints.Len(); i++ {
		attrs := datapoints.At(i).Attributes()
		require.Positive(t, attrs.Len(), "expected attributes for metric %q", metric.Name())

		for key, want := range expectedAttrs {
			got, ok := attrs.Get(key)
			require.Truef(t, ok, "missing attribute %q on metric %q", key, metric.Name())

			if got.Type() == pcommon.ValueTypeStr {
				require.Equalf(t, want, got.Str(), "unexpected value for attribute %q on metric %q", key, metric.Name())
				continue
			}

			require.Equalf(t, want, got.AsString(), "unexpected value for attribute %q on metric %q", key, metric.Name())
		}
	}
}
