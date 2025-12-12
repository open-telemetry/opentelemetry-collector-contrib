// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/metadata"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

// SampleTelemetryData represents a sample telemetry message for testing
type SampleTelemetryData struct {
	NodeID       string               `json:"node_id"`
	Subscription string               `json:"subscription"`
	EncodingPath string               `json:"encoding_path"`
	Interfaces   []InterfaceTelemetry `json:"interfaces"`
}

// InterfaceTelemetry represents telemetry data for a single interface
type InterfaceTelemetry struct {
	Name       string            `json:"name"`
	Statistics map[string]uint64 `json:"statistics"`
}

// TestSampleTelemetryData tests the receiver with sample telemetry data that can be provided by the user
func TestSampleTelemetryData(t *testing.T) {
	// Create a metrics capture consumer
	consumer := &metricsCapture{}

	// Create receiver configuration
	config := createDefaultConfig().(*Config)
	config.NetAddr.Endpoint = "localhost:57404"
	config.NetAddr.Transport = "tcp"

	settings := receiver.Settings{
		ID:                component.NewID(metadata.Type),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}

	ctx := t.Context()
	rcvr, err := createMetricsReceiver(ctx, settings, config, consumer)
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

	// Load sample data (either from file or use built-in sample)
	sampleData := getSampleTelemetryData(t)

	// Send each interface's telemetry data
	for _, interfaceData := range sampleData.Interfaces {
		err := sendInterfaceTelemetryData("localhost:57404", sampleData.NodeID, sampleData.Subscription, sampleData.EncodingPath, interfaceData)
		if err != nil {
			t.Fatalf("Failed to send telemetry for interface %s: %v", interfaceData.Name, err)
		}
	}

	// Wait for data processing
	time.Sleep(200 * time.Millisecond)

	// Analyze the results
	analyzeResults(t, consumer, sampleData)
}

// getSampleTelemetryData returns sample telemetry data (loads from file if available, otherwise uses built-in data)
func getSampleTelemetryData(t *testing.T) SampleTelemetryData {
	// Try to load from sample_telemetry.json file first
	if data, err := loadSampleFromFile(filepath.Join("internal", "testdata", "sample_telemetry.json")); err == nil {
		t.Logf("Loaded sample telemetry data from file: %d interfaces", len(data.Interfaces))
		return data
	}

	// Use built-in realistic sample data based on production observations
	t.Logf("Using built-in sample telemetry data")
	return SampleTelemetryData{
		NodeID:       "PROD-SWITCH-01",
		Subscription: "interface-stats-production",
		EncodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
		Interfaces: []InterfaceTelemetry{
			{
				Name: "GigabitEthernet1/0/1",
				Statistics: map[string]uint64{
					"in-octets":          409369815291,
					"in-unicast-pkts":    388324417,
					"in-broadcast-pkts":  708677,
					"in-multicast-pkts":  1089928,
					"in-discards":        0,
					"in-errors":          0,
					"out-octets":         2785599867,
					"out-unicast-pkts":   237579869,
					"out-broadcast-pkts": 40989872,
					"out-multicast-pkts": 86837950,
					"out-discards":       1947,
					"out-errors":         0,
					"rx-pps":             191,
					"rx-kbps":            2014,
					"tx-pps":             66,
					"tx-kbps":            41,
					"num-flaps":          0,
					"in-crc-errors":      0,
				},
			},
			{
				Name: "TenGigabitEthernet1/0/48",
				Statistics: map[string]uint64{
					"in-octets":          17986853328726,
					"in-unicast-pkts":    22871874518,
					"in-broadcast-pkts":  6952002,
					"in-multicast-pkts":  1775257,
					"in-discards":        0,
					"in-errors":          0,
					"out-octets":         917990342,
					"out-unicast-pkts":   24496004270,
					"out-broadcast-pkts": 42491898,
					"out-multicast-pkts": 116172899,
					"out-discards":       1578153,
					"out-errors":         0,
					"rx-pps":             2332,
					"rx-kbps":            21004,
					"tx-pps":             2389,
					"tx-kbps":            21700,
					"num-flaps":          0,
					"in-crc-errors":      0,
				},
			},
			{
				Name: "VLAN1010",
				Statistics: map[string]uint64{
					"in-octets":          309179668770,
					"in-unicast-pkts":    423933707,
					"in-broadcast-pkts":  14950,
					"in-multicast-pkts":  18818,
					"in-discards":        0,
					"in-errors":          0,
					"out-octets":         3086550873,
					"out-unicast-pkts":   782635848,
					"out-broadcast-pkts": 3219365,
					"out-multicast-pkts": 3611833,
					"out-discards":       245701,
					"out-errors":         0,
					"rx-pps":             237,
					"rx-kbps":            830,
					"tx-pps":             495,
					"tx-kbps":            4801,
					"num-flaps":          0,
					"in-crc-errors":      0,
				},
			},
		},
	}
}

// loadSampleFromFile loads sample telemetry data from a JSON file
func loadSampleFromFile(filename string) (SampleTelemetryData, error) {
	var data SampleTelemetryData

	file, err := os.Open(filename)
	if err != nil {
		return data, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return data, err
	}

	err = json.Unmarshal(bytes, &data)
	return data, err
}

// sendInterfaceTelemetryData sends telemetry data for a specific interface
func sendInterfaceTelemetryData(endpoint, nodeID, subscription, encodingPath string, interfaceData InterfaceTelemetry) error {
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

	// Build telemetry fields from the interface statistics
	var statisticsFields []*pb.TelemetryField
	for statName, statValue := range interfaceData.Statistics {
		statisticsFields = append(statisticsFields, &pb.TelemetryField{
			Name:        statName,
			ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: statValue},
		})
	}

	telemetryData := &pb.Telemetry{
		NodeId:              &pb.Telemetry_NodeIdStr{NodeIdStr: nodeID},
		Subscription:        &pb.Telemetry_SubscriptionIdStr{SubscriptionIdStr: subscription},
		EncodingPath:        encodingPath,
		CollectionId:        1,
		CollectionStartTime: uint64(time.Now().UnixNano()),
		MsgTimestamp:        uint64(time.Now().UnixNano()),
		DataGpbkv: []*pb.TelemetryField{
			{
				Name: "interface",
				Fields: []*pb.TelemetryField{
					{
						Name:        "name",
						ValueByType: &pb.TelemetryField_StringValue{StringValue: interfaceData.Name},
					},
					{
						Name:   "statistics",
						Fields: statisticsFields,
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
	if err != nil && err.Error() != "EOF" {
		return fmt.Errorf("unexpected error receiving response: %w", err)
	}

	return nil
}

// analyzeResults analyzes the captured metrics and provides detailed interface analysis
func analyzeResults(t *testing.T, consumer *metricsCapture, sampleData SampleTelemetryData) {
	metrics := consumer.GetMetrics()
	if len(metrics) == 0 {
		t.Fatal("No metrics were captured")
	}

	t.Logf("\n=== TELEMETRY ANALYSIS REPORT ===")
	t.Logf("Processed %d metric batches from node: %s", len(metrics), sampleData.NodeID)

	// Extract interface statistics
	interfaceStats := make(map[string]map[string]float64)

	for _, md := range metrics {
		resourceMetrics := md.ResourceMetrics()
		for j := 0; j < resourceMetrics.Len(); j++ {
			rm := resourceMetrics.At(j)
			scopeMetrics := rm.ScopeMetrics()

			for k := 0; k < scopeMetrics.Len(); k++ {
				sm := scopeMetrics.At(k)

				// Find interface name for this batch
				var batchInterfaceName string
				metrics := sm.Metrics()
				for l := 0; l < metrics.Len(); l++ {
					metric := metrics.At(l)
					if metric.Name() == "cisco.interface.name_info" {
						gauge := metric.Gauge()
						if gauge.DataPoints().Len() > 0 {
							dp := gauge.DataPoints().At(0)
							if val, exists := dp.Attributes().Get("value"); exists {
								batchInterfaceName = val.Str()
								break
							}
						}
					}
				}

				// Associate all metrics with this interface
				if batchInterfaceName != "" {
					if interfaceStats[batchInterfaceName] == nil {
						interfaceStats[batchInterfaceName] = make(map[string]float64)
					}

					for l := 0; l < metrics.Len(); l++ {
						metric := metrics.At(l)
						metricName := metric.Name()

						var metricValue float64
						if metric.Type() == pmetric.MetricTypeGauge {
							gauge := metric.Gauge()
							if gauge.DataPoints().Len() > 0 {
								metricValue = gauge.DataPoints().At(0).DoubleValue()
							}
						}

						interfaceStats[batchInterfaceName][metricName] = metricValue
					}
				}
			}
		}
	}

	// Analyze traffic patterns
	t.Logf("\n=== INTERFACE TRAFFIC ANALYSIS ===")

	var interfaces []struct {
		name       string
		totalPkts  uint64
		totalBytes uint64
		bandwidth  uint64
	}

	for interfaceName, stats := range interfaceStats {
		rxPkts := uint64(stats["cisco.interface.statistics.in-unicast-pkts"])
		txPkts := uint64(stats["cisco.interface.statistics.out-unicast-pkts"])
		rxBytes := uint64(stats["cisco.interface.statistics.in-octets"])
		txBytes := uint64(stats["cisco.interface.statistics.out-octets"])
		rxKbps := uint64(stats["cisco.interface.statistics.rx-kbps"])
		txKbps := uint64(stats["cisco.interface.statistics.tx-kbps"])

		totalPkts := rxPkts + txPkts
		totalBytes := rxBytes + txBytes
		totalBandwidth := rxKbps + txKbps

		interfaces = append(interfaces, struct {
			name       string
			totalPkts  uint64
			totalBytes uint64
			bandwidth  uint64
		}{interfaceName, totalPkts, totalBytes, totalBandwidth})
		t.Logf("\nInterface: %s", interfaceName)
		t.Logf("   RX: %s packets, %s bytes (%d kbps)",
			formatLargeNumber(rxPkts), formatBytes(rxBytes), rxKbps)
		t.Logf("   TX: %s packets, %s bytes (%d kbps)",
			formatLargeNumber(txPkts), formatBytes(txBytes), txKbps)
		t.Logf("   Total: %s packets, %s bytes (Raw: RX=%d, TX=%d, Total=%d)",
			formatLargeNumber(totalPkts), formatBytes(totalBytes), int(rxPkts), int(txPkts), int(totalPkts))
	}

	// Find busiest interfaces
	t.Logf("\n=== TOP TRAFFIC INTERFACES ===")

	// Sort by total packets (descending order)
	for i := 0; i < len(interfaces)-1; i++ {
		for j := 0; j < len(interfaces)-i-1; j++ {
			if interfaces[j].totalPkts < interfaces[j+1].totalPkts {
				interfaces[j], interfaces[j+1] = interfaces[j+1], interfaces[j]
			}
		}
	}

	for i, iface := range interfaces {
		if i >= 3 {
			break
		} // Show top 3
		t.Logf("   %d. %s - %s packets (%s bytes, %d kbps total)",
			i+1, iface.name, formatLargeNumber(iface.totalPkts),
			formatBytes(iface.totalBytes), iface.bandwidth)
	}

	// Validate we received all expected interfaces
	t.Logf("\n=== VALIDATION RESULTS ===")
	for _, expectedInterface := range sampleData.Interfaces {
		if _, exists := interfaceStats[expectedInterface.Name]; exists {
			t.Logf("   PASS %s: Telemetry received successfully", expectedInterface.Name)
		} else {
			t.Errorf("   FAIL %s: Missing telemetry data", expectedInterface.Name)
		}
	}
}

// Helper functions for formatting
func formatLargeNumber(n uint64) string {
	switch {
	case n >= 1e12:
		return fmt.Sprintf("%.2fT", float64(n)/1e12)
	case n >= 1e9:
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.2fK", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

func formatBytes(bytes uint64) string {
	switch {
	case bytes >= 1<<40:
		return fmt.Sprintf("%.2fTB", float64(bytes)/(1<<40))
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2fGB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.2fMB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.2fKB", float64(bytes)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", bytes)
}
