// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
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

// convertToOTELMetrics maps Cisco KV-GPB data to OTLP using a Telegraf-inspired
// approach: extract identifiers (tags) first, then emit measurements.
func (s *grpcService) convertToOTELMetrics(telemetry *pb.Telemetry) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	resAttrs := rm.Resource().Attributes()
	resAttrs.PutStr("cisco.node_id", telemetry.GetNodeIdStr())
	resAttrs.PutStr("cisco.encoding_path", telemetry.EncodingPath)

	sm := rm.ScopeMetrics().AppendEmpty()
	timestamp := pcommon.Timestamp(telemetry.MsgTimestamp * 1000000)

	// Process each entry in DataGpbkv as a distinct row/object.
	for _, field := range telemetry.DataGpbkv {
		// Step 1: Initialize context bag with global metadata.
		ctxBag := map[string]string{
			"node_id": telemetry.GetNodeIdStr(),
		}

		// Step 2: Pre-scan the entire tree for keys/identifiers (Telegraf logic).
		// This ensures sibling branches like 'admin-status' can access 'interface-name'.
		s.extractKeys(field, ctxBag)

		// Step 3: Walk the tree again to emit actual metrics using the enriched context.
		s.emitMetrics(sm, field, "", timestamp, ctxBag)
	}

	return metrics
}

// extractKeys recursively scans for string values that serve as identifiers.
func (s *grpcService) extractKeys(field *pb.TelemetryField, ctxBag map[string]string) {
	val := formatValueToString(field)
	if val != "" {
		// If it's a string value, it's likely a dimension/tag.
		if _, ok := field.ValueByType.(*pb.TelemetryField_StringValue); ok {
			ctxBag[field.Name] = val

			// Common Cisco naming normalization.
			lowName := strings.ToLower(field.Name)
			if lowName == "name" || lowName == "interface-name" {
				ctxBag["interface"] = val
			}
		}
	}
	for _, child := range field.Fields {
		s.extractKeys(child, ctxBag)
	}
}

// emitMetrics processes numerical values and emits OTLP metrics with the full context bag.
func (s *grpcService) emitMetrics(sm pmetric.ScopeMetrics, field *pb.TelemetryField, pathPrefix string, timestamp pcommon.Timestamp, ctxBag map[string]string) {
	currentPath := pathPrefix
	if currentPath != "" {
		currentPath += "."
	}
	currentPath += field.Name

	// Only emit metrics for leaf nodes (values) that are NOT in the 'keys' branch.
	if field.ValueByType != nil && len(field.Fields) == 0 && !strings.HasPrefix(currentPath, "keys") {
		m := sm.Metrics().AppendEmpty()
		cleanName := strings.TrimPrefix(currentPath, "content.")
		metricLabels := cloneCtxBag(ctxBag)

		if strVal, ok := field.ValueByType.(*pb.TelemetryField_StringValue); ok {
			// Step/Info metrics for string states (e.g., Up/Down).
			createStepMetric(m, cleanName, strVal.StringValue, timestamp, metricLabels)
		} else {
			// Numeric metrics for counters and gauges.
			createNumericMetric(m, cleanName, getNumericValue(field), timestamp, nil, metricLabels)
		}
	}

	for _, child := range field.Fields {
		s.emitMetrics(sm, child, currentPath, timestamp, ctxBag)
	}
}

// createNumericMetric populates a NumberDataPoint.
func createNumericMetric(m pmetric.Metric, name string, val float64, ts pcommon.Timestamp, yType *internal.YANGDataType, ctx map[string]string) {
	m.SetName("cisco." + name)
	if yType != nil && yType.IsCounterType() {
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp := sum.DataPoints().AppendEmpty()
		dp.SetDoubleValue(val)
		dp.SetTimestamp(ts)
		applyCtxBag(dp.Attributes(), ctx)
	} else {
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetDoubleValue(val)
		dp.SetTimestamp(ts)
		applyCtxBag(dp.Attributes(), ctx)
	}
}

// createStepMetric creates an "Info" metric where the actual value is an attribute.
func createStepMetric(m pmetric.Metric, name, val string, ts pcommon.Timestamp, ctx map[string]string) {
	m.SetName("cisco." + name + "_info")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0)
	dp.SetTimestamp(ts)
	dp.Attributes().PutStr("value", val)
	applyCtxBag(dp.Attributes(), ctx)
}

// getNumericValue extracts float64 from various protobuf types.
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

// formatValueToString converts protobuf field values to string for labels.
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

func cloneCtxBag(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	maps.Copy(in, out)
	return out
}

func applyCtxBag(attrs pcommon.Map, ctx map[string]string) {
	for k, v := range ctx {
		if v != "" {
			attrs.PutStr(k, v)
		}
	}
}
