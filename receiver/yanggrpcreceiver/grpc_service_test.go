// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

func TestGrpcService_ProcessTelemetryData(t *testing.T) {
	// Create a test receiver
	config := createValidTestConfig()
	config.NetAddr.Endpoint = "localhost:0" // Use a random port

	mockConsumer := &consumertest.MetricsSink{}
	settings := createTestSettings()

	ctr, err := createMetricsReceiver(t.Context(), settings, config, mockConsumer)
	if err != nil {
		t.Fatalf("Failed to create receiver: %v", err)
	}

	// Create gRPC service with YANG parser
	yangParser := internal.NewYANGParser()
	yangParser.LoadBuiltinModules()
	rfcYangParser := internal.NewRFC6020Parser()
	service := &grpcService{
		receiver:      ctr.(*yangReceiver),
		yangParser:    yangParser,
		rfcYangParser: rfcYangParser,
	}

	// Create test telemetry data
	telemetry := &pb.Telemetry{
		NodeId: &pb.Telemetry_NodeIdStr{
			NodeIdStr: "test-node-1",
		},
		Subscription: &pb.Telemetry_SubscriptionIdStr{
			SubscriptionIdStr: "test-subscription",
		},
		EncodingPath: "/interfaces-ios-xe-oper:interfaces/interface/statistics",
		CollectionId: 12345,
		MsgTimestamp: uint64(time.Now().UnixMilli()),
		DataGpbkv: []*pb.TelemetryField{
			{
				Name: "interface",
				Fields: []*pb.TelemetryField{
					{
						Name: "name",
						ValueByType: &pb.TelemetryField_StringValue{
							StringValue: "GigabitEthernet0/0/1",
						},
					},
					{
						Name: "statistics",
						Fields: []*pb.TelemetryField{
							{
								Name: "rx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{
									Uint64Value: 1234567,
								},
							},
							{
								Name: "tx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{
									Uint64Value: 2345678,
								},
							},
						},
					},
				},
			},
		},
	}

	// Serialize the telemetry data
	data, err := proto.Marshal(telemetry)
	if err != nil {
		t.Fatalf("Failed to marshal telemetry: %v", err)
	}

	// Create MdtDialoutArgs
	req := &pb.MdtDialoutArgs{
		ReqId: 1,
		Data:  data,
	}

	// Process the telemetry data
	err = service.processTelemetryData(req)
	if err != nil {
		t.Fatalf("Failed to process telemetry data: %v", err)
	}

	// Check that metrics were consumed
	if len(mockConsumer.AllMetrics()) == 0 {
		t.Error("Expected metrics to be consumed, but got none")
	}

	// Verify the metrics content
	metrics := mockConsumer.AllMetrics()[0]
	if metrics.ResourceMetrics().Len() == 0 {
		t.Error("Expected resource metrics, but got none")
	}

	resourceMetrics := metrics.ResourceMetrics().At(0)
	resource := resourceMetrics.Resource()

	// Check resource attributes
	nodeID, ok := resource.Attributes().Get("cisco.node_id")
	if !ok || nodeID.Str() != "test-node-1" {
		t.Errorf("Expected node_id 'test-node-1', got %v", nodeID)
	}

	encodingPath, ok := resource.Attributes().Get("cisco.encoding_path")
	if !ok || encodingPath.Str() != "/interfaces-ios-xe-oper:interfaces/interface/statistics" {
		t.Errorf("Expected encoding_path, got %v", encodingPath)
	}

	// Check that we have scope metrics with actual metrics
	if resourceMetrics.ScopeMetrics().Len() == 0 {
		t.Error("Expected scope metrics, but got none")
	}

	scopeMetrics := resourceMetrics.ScopeMetrics().At(0)
	if scopeMetrics.Metrics().Len() == 0 {
		t.Error("Expected metrics, but got none")
	}

	t.Logf("Successfully processed telemetry data with %d metrics", scopeMetrics.Metrics().Len())
}

func TestKvGPBDataParsing(t *testing.T) {
	tests := []struct {
		name     string
		field    *pb.TelemetryField
		expected string
	}{
		{
			name: "uint64_value",
			field: &pb.TelemetryField{
				Name: "rx-pkts",
				ValueByType: &pb.TelemetryField_Uint64Value{
					Uint64Value: 1234567,
				},
			},
			expected: "cisco.rx-pkts",
		},
		{
			name: "string_value",
			field: &pb.TelemetryField{
				Name: "interface-name",
				ValueByType: &pb.TelemetryField_StringValue{
					StringValue: "GigabitEthernet0/0/1",
				},
			},
			expected: "cisco.interface-name_info",
		},
		{
			name: "bool_value",
			field: &pb.TelemetryField{
				Name: "enabled",
				ValueByType: &pb.TelemetryField_BoolValue{
					BoolValue: true,
				},
			},
			expected: "cisco.enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			config := &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr:              confignet.NewDefaultAddrConfig(),
					MaxConcurrentStreams: 100,
				},
				YANG:     YANGConfig{},
				Security: SecurityConfig{},
			}
			mockConsumer := &consumertest.MetricsSink{}
			settings := receiver.Settings{
				TelemetrySettings: component.TelemetrySettings{
					Logger:        zap.NewNop(),
					MeterProvider: noop.NewMeterProvider(),
				},
			}

			ctr, err := createMetricsReceiver(t.Context(), settings, config, mockConsumer)
			if err != nil {
				t.Fatalf("Failed to create receiver: %v", err)
			}

			yangParser := internal.NewYANGParser()
			yangParser.LoadBuiltinModules()
			service := &grpcService{
				receiver:   ctr.(*yangReceiver),
				yangParser: yangParser,
			}

			// Create test telemetry
			telemetry := &pb.Telemetry{
				NodeId:       &pb.Telemetry_NodeIdStr{NodeIdStr: "test-node"},
				EncodingPath: "/test/path",
				MsgTimestamp: uint64(time.Now().UnixMilli()),
				DataGpbkv:    []*pb.TelemetryField{tt.field},
			}

			data, err := proto.Marshal(telemetry)
			if err != nil {
				t.Fatalf("Failed to marshal telemetry: %v", err)
			}

			req := &pb.MdtDialoutArgs{
				ReqId: 1,
				Data:  data,
			}

			// Process
			err = service.processTelemetryData(req)
			if err != nil {
				t.Fatalf("Failed to process telemetry data: %v", err)
			}

			// Verify
			if len(mockConsumer.AllMetrics()) == 0 {
				t.Error("Expected metrics to be consumed")
				return
			}

			metrics := mockConsumer.AllMetrics()[0]
			scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)

			if scopeMetrics.Metrics().Len() == 0 {
				t.Error("Expected at least one metric")
				return
			}

			metric := scopeMetrics.Metrics().At(0)
			if metric.Name() != tt.expected {
				t.Errorf("Expected metric name %s, got %s", tt.expected, metric.Name())
			}
		})
	}
}
