// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/metadata"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

// metricsCapture captures metrics for detailed analysis
type metricsCapture struct {
	mutex   sync.Mutex
	metrics []pmetric.Metrics
}

func (*metricsCapture) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *metricsCapture) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Clone the metrics to avoid data races
	cloned := pmetric.NewMetrics()
	md.CopyTo(cloned)
	m.metrics = append(m.metrics, cloned)
	return nil
}

func (m *metricsCapture) GetMetrics() []pmetric.Metrics {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]pmetric.Metrics, len(m.metrics))
	copy(result, m.metrics)
	return result
}

func (m *metricsCapture) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.metrics = nil
}

func TestDetailedTelemetryValidation(t *testing.T) {
	// Create a metrics capture consumer
	csmr := &metricsCapture{}

	// Create receiver configuration
	config := createDefaultConfig().(*Config)

	config.NetAddr.Endpoint = "localhost:57403"
	config.NetAddr.Transport = "tcp"

	settings := receiver.Settings{
		ID:                component.NewID(metadata.Type),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}

	ctx := t.Context()
	rcvr, err := createMetricsReceiver(ctx, settings, config, csmr)
	if err != nil {
		t.Fatalf("Failed to create receiver: %v", err)
	}

	err = rcvr.Start(ctx, componenttest.NewNopHost())
	if err != nil {
		t.Fatalf("Failed to start receiver: %v", err)
	}
	defer func() {
		assert.NoError(t, rcvr.Shutdown(ctx))
	}()

	time.Sleep(50 * time.Millisecond)

	// Send realistic telemetry data with multiple interfaces
	testCases := []struct {
		name          string
		interfaceName string
		rxPkts        uint64
		txPkts        uint64
		rxBytes       uint64
		txBytes       uint64
	}{
		{
			name:          "GigabitEthernet1/0/1",
			interfaceName: "GigabitEthernet1/0/1",
			rxPkts:        1234567890,
			txPkts:        987654321,
			rxBytes:       5000000000,
			txBytes:       3000000000,
		},
		{
			name:          "VLAN1010",
			interfaceName: "VLAN1010",
			rxPkts:        555666777,
			txPkts:        333444555,
			rxBytes:       2500000000,
			txBytes:       1800000000,
		},
		{
			name:          "TenGigabitEthernet1/0/48",
			interfaceName: "TenGigabitEthernet1/0/48",
			rxPkts:        9876543210,
			txPkts:        5432109876,
			rxBytes:       15000000000,
			txBytes:       12000000000,
		},
	}

	for _, tc := range testCases {
		err := sendDetailedTelemetryData("localhost:57403", "CISCO-TEST-SWITCH", tc.interfaceName, tc.rxPkts, tc.txPkts, tc.rxBytes, tc.txBytes)
		if err != nil {
			t.Fatalf("Failed to send telemetry for %s: %v", tc.name, err)
		}
	}

	// Wait for data processing
	time.Sleep(200 * time.Millisecond)

	// Analyze captured metrics
	metrics := csmr.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("No metrics were captured")
	}

	t.Logf("Captured %d metric batches", len(metrics))

	// Detailed validation - correlate interface names with statistics
	interfaceStats := make(map[string]map[string]float64)

	for i, md := range metrics {
		t.Logf("\n=== Analyzing Metric Batch %d ===", i+1)

		resourceMetrics := md.ResourceMetrics()
		for j := 0; j < resourceMetrics.Len(); j++ {
			rm := resourceMetrics.At(j)

			// Extract resource attributes
			attrs := rm.Resource().Attributes()
			nodeID := ""
			subscriptionID := ""
			encodingPath := ""

			if val, exists := attrs.Get("cisco.node_id"); exists {
				nodeID = val.Str()
			}
			if val, exists := attrs.Get("cisco.subscription_id"); exists {
				subscriptionID = val.Str()
			}
			if val, exists := attrs.Get("cisco.encoding_path"); exists {
				encodingPath = val.Str()
			}

			t.Logf("Resource - Node ID: %s, Subscription: %s, Path: %s", nodeID, subscriptionID, encodingPath)

			// Process scope metrics - find interface name first
			scopeMetrics := rm.ScopeMetrics()
			for k := 0; k < scopeMetrics.Len(); k++ {
				sm := scopeMetrics.At(k)
				t.Logf("  Scope: %s (%d metrics)", sm.Scope().Name(), sm.Metrics().Len())

				// First pass: find the interface name from the name_info metric
				batchInterfaceName := ""
				metrics := sm.Metrics()
				for l := 0; l < metrics.Len(); l++ {
					metric := metrics.At(l)
					if metric.Name() == "cisco.interface.name_info" {
						gauge := metric.Gauge()
						dataPoints := gauge.DataPoints()
						if dataPoints.Len() > 0 {
							dp := dataPoints.At(0)
							if val, exists := dp.Attributes().Get("value"); exists {
								batchInterfaceName = val.Str()
								t.Logf("    Found interface name for batch: %s", batchInterfaceName)
								break
							}
						}
					}
				}

				// Second pass: associate all metrics in this batch with the interface
				if batchInterfaceName != "" {
					if interfaceStats[batchInterfaceName] == nil {
						interfaceStats[batchInterfaceName] = make(map[string]float64)
					}

					for l := 0; l < metrics.Len(); l++ {
						metric := metrics.At(l)
						metricName := metric.Name()

						var metricValue float64
						switch metric.Type() {
						case pmetric.MetricTypeGauge:
							gauge := metric.Gauge()
							dataPoints := gauge.DataPoints()
							if dataPoints.Len() > 0 {
								metricValue = dataPoints.At(0).DoubleValue()
							}
						case pmetric.MetricTypeSum:
							sum := metric.Sum()
							dataPoints := sum.DataPoints()
							if dataPoints.Len() > 0 {
								metricValue = dataPoints.At(0).DoubleValue()
							}
						}

						interfaceStats[batchInterfaceName][metricName] = metricValue
						t.Logf("    %s [%s] = %.0f", metricName, batchInterfaceName, metricValue)
					}
				}
			}
		}
	}

	// Validate expected interface data
	t.Logf("\n=== Interface Statistics Summary ===")
	for interfaceName, stats := range interfaceStats {
		t.Logf("\nInterface: %s", interfaceName)

		// Check for expected metrics
		expectedMetrics := []string{
			"cisco.interface.statistics.rx-pkts",
			"cisco.interface.statistics.tx-pkts",
			"cisco.interface.statistics.rx-bytes",
			"cisco.interface.statistics.tx-bytes",
		}

		for _, expectedMetric := range expectedMetrics {
			if value, exists := stats[expectedMetric]; exists {
				t.Logf("  PASS %s: %.0f", expectedMetric, value)
			} else {
				t.Logf("  MISSING: %s", expectedMetric)
			}
		}

		// Find highest traffic interface
		rxPkts := stats["cisco.interface.statistics.rx-pkts"]
		txPkts := stats["cisco.interface.statistics.tx-pkts"]
		totalPkts := rxPkts + txPkts
		t.Logf("  Total Packets: %.0f (RX: %.0f, TX: %.0f)", totalPkts, rxPkts, txPkts)
	}

	// Find interface with most traffic
	var highestTrafficInterface string
	var highestTrafficCount float64

	for interfaceName, stats := range interfaceStats {
		rxPkts := stats["cisco.interface.statistics.rx-pkts"]
		txPkts := stats["cisco.interface.statistics.tx-pkts"]
		totalPkts := rxPkts + txPkts

		if totalPkts > highestTrafficCount {
			highestTrafficCount = totalPkts
			highestTrafficInterface = interfaceName
		}
	}

	if highestTrafficInterface != "" {
		t.Logf("\nHighest Traffic Interface: %s (%.0f total packets)", highestTrafficInterface, highestTrafficCount)

		// Validate this is TenGigabitEthernet1/0/48 (our test case with highest values)
		if highestTrafficInterface == "TenGigabitEthernet1/0/48" {
			t.Logf("PASS: Correctly identified highest traffic interface")
		} else {
			t.Errorf("FAIL: Expected TenGigabitEthernet1/0/48 to have highest traffic, but got %s", highestTrafficInterface)
		}
	}

	// Validate we received metrics for all test interfaces
	expectedInterfaces := []string{"GigabitEthernet1/0/1", "VLAN1010", "TenGigabitEthernet1/0/48"}
	for _, expectedInterface := range expectedInterfaces {
		if _, exists := interfaceStats[expectedInterface]; exists {
			t.Logf("PASS: Found telemetry for interface: %s", expectedInterface)
		} else {
			t.Errorf("FAIL: Missing telemetry for interface: %s", expectedInterface)
		}
	}
}

// sendDetailedTelemetryData sends telemetry data for a specific interface with detailed metrics
func sendDetailedTelemetryData(endpoint, nodeID, interfaceName string, rxPkts, txPkts, rxBytes, txBytes uint64) error {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to receiver: %w", err)
	}
	defer conn.Close()

	client := pb.NewGRPCMdtDialoutClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.MdtDialout(ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Create comprehensive telemetry data
	telemetryData := &pb.Telemetry{
		NodeId:              &pb.Telemetry_NodeIdStr{NodeIdStr: nodeID},
		Subscription:        &pb.Telemetry_SubscriptionIdStr{SubscriptionIdStr: "interface-stats"},
		EncodingPath:        "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
		CollectionId:        1,
		CollectionStartTime: uint64(time.Now().UnixNano()),
		MsgTimestamp:        uint64(time.Now().UnixNano()),
		DataGpbkv: []*pb.TelemetryField{
			{
				Name: "interface",
				Fields: []*pb.TelemetryField{
					{
						Name:        "name",
						ValueByType: &pb.TelemetryField_StringValue{StringValue: interfaceName},
					},
					{
						Name: "statistics",
						Fields: []*pb.TelemetryField{
							{
								Name:        "rx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: rxPkts},
							},
							{
								Name:        "tx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: txPkts},
							},
							{
								Name:        "rx-bytes",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: rxBytes},
							},
							{
								Name:        "tx-bytes",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: txBytes},
							},
							{
								Name:        "rx-pps",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: rxPkts / 60}, // Approx rate
							},
							{
								Name:        "tx-pps",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: txPkts / 60}, // Approx rate
							},
							{
								Name:        "rx-kbps",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: (rxBytes * 8) / 1000 / 60}, // Approx kbps
							},
							{
								Name:        "tx-kbps",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: (txBytes * 8) / 1000 / 60}, // Approx kbps
							},
							{
								Name:        "in-errors",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 0},
							},
							{
								Name:        "out-errors",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 0},
							},
							{
								Name:        "in-discards",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 0},
							},
							{
								Name:        "out-discards",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 0},
							},
						},
					},
				},
			},
		},
	}

	telemetryBytes, err := proto.Marshal(telemetryData)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry data: %w", err)
	}

	args := &pb.MdtDialoutArgs{
		ReqId: 1,
		Data:  telemetryBytes,
	}

	err = stream.Send(args)
	if err != nil {
		return fmt.Errorf("failed to send telemetry data: %w", err)
	}

	err = stream.CloseSend()
	if err != nil {
		return fmt.Errorf("failed to close send stream: %w", err)
	}

	_, err = stream.Recv()
	if err != nil && errors.Is(err, io.EOF) {
		return fmt.Errorf("unexpected error receiving response: %w", err)
	}

	return nil
}
