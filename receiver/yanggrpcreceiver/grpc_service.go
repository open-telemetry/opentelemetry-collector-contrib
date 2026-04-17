// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

// grpcService handles Cisco gRPC Dial-out telemetry streams.
type grpcService struct {
	pb.UnimplementedGRPCMdtDialoutServer
	receiver   *yangReceiver
	yangParser *internal.YANGParser
}

// MdtDialout processes the bidirectional gRPC stream.
func (s *grpcService) MdtDialout(stream pb.GRPCMdtDialout_MdtDialoutServer) error {
	s.receiver.settings.Logger.Info("New Cisco telemetry session established")
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		if err := s.processTelemetryData(req); err != nil {
			s.receiver.settings.Logger.Error("Failed to process telemetry data", zap.Error(err))
		}
	}
}

// processTelemetryData unmarshals the GPBKV payload and triggers OTLP conversion.
func (s *grpcService) processTelemetryData(req *pb.MdtDialoutArgs) error {
	telemetryMsg := &pb.Telemetry{}
	if err := proto.Unmarshal(req.Data, telemetryMsg); err != nil {
		return fmt.Errorf("failed to unmarshal telemetry payload: %w", err)
	}

	metrics := s.convertToOTELMetrics(telemetryMsg)
	return s.receiver.consumer.ConsumeMetrics(context.Background(), metrics)
}

// convertToOTELMetrics maps Cisco KV-GPB data to OTLP using a Two-Pass approach.
func (s *grpcService) convertToOTELMetrics(telemetry *pb.Telemetry) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	// Resource attributes are global to the device/node.
	resAttrs := rm.Resource().Attributes()
	resAttrs.PutStr("cisco.node_id", telemetry.GetNodeIdStr())
	resAttrs.PutStr("cisco.encoding_path", telemetry.EncodingPath)

	sm := rm.ScopeMetrics().AppendEmpty()
	timestamp := pcommon.Timestamp(telemetry.MsgTimestamp * 1000000)

	for _, field := range telemetry.DataGpbkv {
		// Log the raw structure for deep debugging if needed.
		s.receiver.settings.Logger.Debug("Processing GPBKV Field", zap.String("name", field.Name))

		// Step 1: Initialize context bag with the node ID.
		ctxBag := map[string]string{
			"node_id": telemetry.GetNodeIdStr(),
		}

		// Step 2: Extract identifying keys only from the "keys" branch.
		// This prevents metrics from leaking into labels.
		s.extractKeysOnly(field, ctxBag)

		// Step 3: Walk the "content" branch only to emit measurements.
		// We pass the ctxBag to attach labels to every data point.
		s.emitMetricsOnly(sm, field, "", timestamp, ctxBag)
	}

	return metrics
}

// extractKeysOnly specifically targets the "keys" branch to populate the ctxBag (Labels).
func (s *grpcService) extractKeysOnly(field *pb.TelemetryField, ctxBag map[string]string) {
	// If we hit the "keys" branch, extract all its children as attributes.
	if field.Name == "keys" {
		for _, subField := range field.Fields {
			val := formatValueToString(subField)
			if val != "" {
				ctxBag[subField.Name] = val
				// Add standard "interface" alias for convenience in Splunk/Prometheus.
				lowName := strings.ToLower(subField.Name)
				if lowName == "name" || lowName == "cname" || lowName == "interface-name" {
					ctxBag["interface"] = val
				}
			}
		}
		return
	}

	// Recursive search for the "keys" branch in the tree.
	for _, child := range field.Fields {
		s.extractKeysOnly(child, ctxBag)
	}
}

// emitMetricsOnly scans the "content" branch to generate OTLP Gauges and Sums.
func (s *grpcService) emitMetricsOnly(sm pmetric.ScopeMetrics, field *pb.TelemetryField, pathPrefix string, timestamp pcommon.Timestamp, ctxBag map[string]string) {
	// Skip the keys branch as it's already handled by extractKeysOnly.
	if field.Name == "keys" {
		return
	}

	// Build the path (e.g., statistics.in-octets).
	// We strip "content" prefix for cleaner metric names.
	currentPath := pathPrefix
	if field.Name != "content" {
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += field.Name
	}

	// If it's a leaf node with a value, emit it as a metric.
	if field.ValueByType != nil && len(field.Fields) == 0 {
		m := sm.Metrics().AppendEmpty()

		if strVal, ok := field.ValueByType.(*pb.TelemetryField_StringValue); ok {
			// Handle string states (up/down) as info metrics (value=1, state in attribute).
			createStepMetric(m, currentPath, strVal.StringValue, timestamp, ctxBag)
		} else {
			// Handle numeric values (counters/gauges).
			createNumericMetric(m, currentPath, getNumericValue(field), timestamp, ctxBag)
		}
	}

	// Recursive walk through sub-branches (like 'statistics', 'ether-stats', etc.)
	for _, child := range field.Fields {
		s.emitMetricsOnly(sm, child, currentPath, timestamp, ctxBag)
	}
}

// createNumericMetric populates a Gauge or Sum data point.
func createNumericMetric(m pmetric.Metric, name string, val float64, ts pcommon.Timestamp, ctx map[string]string) {
	m.SetName("cisco." + name)

	// Note: Without a full YANG dictionary, we default to Gauges.
	// Sums/Counters can be refined here if name contains "octets", "pkts" or "errors".
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(val)
	dp.SetTimestamp(ts)
	applyCtxBag(dp.Attributes(), ctx)
}

// createStepMetric creates an "Info" metric where the state is stored as a 'value' attribute.
func createStepMetric(m pmetric.Metric, name, val string, ts pcommon.Timestamp, ctx map[string]string) {
	m.SetName("cisco." + name + "_info")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0)
	dp.SetTimestamp(ts)

	// Add the actual status string as an attribute.
	dp.Attributes().PutStr("value", val)
	applyCtxBag(dp.Attributes(), ctx)
}

// getNumericValue extracts float64 from various Protobuf numeric types.
func getNumericValue(f *pb.TelemetryField) float64 {
	switch v := f.ValueByType.(type) {
	case *pb.TelemetryField_Uint32Value:
		return float64(v.Uint32Value)
	case *pb.TelemetryField_Uint64Value:
		return float64(v.Uint64Value)
	case *pb.TelemetryField_Sint32Value:
		return float64(v.Sint32Value)
	case *pb.TelemetryField_Sint64Value:
		return float64(v.Sint64Value)
	case *pb.TelemetryField_DoubleValue:
		return v.DoubleValue
	case *pb.TelemetryField_FloatValue:
		return float64(v.FloatValue)
	case *pb.TelemetryField_BoolValue:
		if v.BoolValue {
			return 1.0
		}
		return 0.0
	}
	return 0.0
}

// formatValueToString converts leaf values to strings specifically for labels.
func formatValueToString(f *pb.TelemetryField) string {
	if f.ValueByType == nil {
		return ""
	}
	switch v := f.ValueByType.(type) {
	case *pb.TelemetryField_StringValue:
		return v.StringValue
	case *pb.TelemetryField_Uint32Value:
		return strconv.FormatUint(uint64(v.Uint32Value), 10)
	case *pb.TelemetryField_Uint64Value:
		return strconv.FormatUint(v.Uint64Value, 10)
	case *pb.TelemetryField_Sint32Value:
		return strconv.FormatInt(int64(v.Sint32Value), 10)
	case *pb.TelemetryField_Sint64Value:
		return strconv.FormatInt(v.Sint64Value, 10)
	}
	return ""
}

// applyCtxBag efficiently transfers labels from the map to the OTLP attributes map.
func applyCtxBag(attrs pcommon.Map, ctx map[string]string) {
	for k, v := range ctx {
		if v != "" {
			attrs.PutStr(k, v)
		}
	}
}
