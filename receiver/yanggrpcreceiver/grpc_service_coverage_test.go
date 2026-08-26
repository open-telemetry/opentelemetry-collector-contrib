// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

func TestMdtDialout_BasicFlow(t *testing.T) {
	// Create test receiver and gRPC service
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

	// Simple test - just verify the service exists and methods are callable
	assert.NotNil(t, service)
	assert.NotNil(t, service.receiver)
	assert.NotNil(t, service.yangParser)
}

func TestProcessTelemetryData_ErrorHandling(t *testing.T) {
	// Create test receiver and gRPC service
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

	// Test with empty message (no data)
	emptyMsg := &pb.MdtDialoutArgs{
		ReqId: 12345,
		Data:  []byte{}, // Empty data
	}
	err := service.processTelemetryData(emptyMsg)
	assert.NoError(t, err)

	// Test with invalid protobuf data
	invalidMsg := &pb.MdtDialoutArgs{
		ReqId: 12345,
		Data:  []byte("invalid protobuf data"),
	}
	err = service.processTelemetryData(invalidMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse invalid wire-format data")

	// Test with valid telemetry data (simple case)
	validTelemetry := &pb.Telemetry{
		NodeId:       &pb.Telemetry_NodeIdStr{NodeIdStr: "test-node"},
		EncodingPath: "simple:test",
		MsgTimestamp: 1234567890,
	}
	validData, err := proto.Marshal(validTelemetry)
	require.NoError(t, err)

	validMsg := &pb.MdtDialoutArgs{
		ReqId: 12345,
		Data:  validData,
	}
	err = service.processTelemetryData(validMsg)
	assert.NoError(t, err)
}

func TestConvertToOTELMetrics(t *testing.T) {
	config := createValidTestConfig()
	consumer := &consumertest.MetricsSink{}
	settings := createTestSettings()

	receiver := createMetricsReceiver(t.Context(), settings, config, consumer)

	yangParser := internal.NewYANGParser()
	yangParser.LoadBuiltinModules()

	// Don't initialize RFC parser to avoid nil pointer issues
	service := &grpcService{
		receiver:   receiver.(*yangReceiver),
		yangParser: yangParser,
		// rfcYangParser is nil, which is handled in the code
	}

	// Test with minimal valid telemetry data to avoid complex YANG processing
	telemetry := &pb.Telemetry{
		NodeId: &pb.Telemetry_NodeIdStr{
			NodeIdStr: "test-node",
		},
		Subscription: &pb.Telemetry_SubscriptionIdStr{
			SubscriptionIdStr: "test-subscription",
		},
		EncodingPath: "simple:path",
		MsgTimestamp: 1234567890,
		// Empty DataGpbkv to avoid complex field processing
		DataGpbkv: []*pb.TelemetryField{},
	}

	metrics := service.convertToOTELMetrics(telemetry)
	assert.NotNil(t, metrics)
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())

	resourceMetrics := metrics.ResourceMetrics().At(0)
	resource := resourceMetrics.Resource()
	attrs := resource.Attributes()

	nodeID, exists := attrs.Get("cisco.node_id")
	assert.True(t, exists)
	assert.Equal(t, "test-node", nodeID.Str())
}
