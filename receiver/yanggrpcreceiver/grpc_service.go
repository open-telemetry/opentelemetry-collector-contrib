// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

// grpcService implements the Cisco MDT gRPC service
type grpcService struct {
	pb.UnimplementedGRPCMdtDialoutServer
	receiver      *yangReceiver
	yangParser    *internal.YANGParser
	rfcYangParser *internal.RFC6020Parser
}

// MdtDialout handles the bidirectional streaming gRPC call from Cisco devices
func (s *grpcService) MdtDialout(stream grpc.BidiStreamingServer[pb.MdtDialoutArgs, pb.MdtDialoutArgs]) error {
	ctx := context.Background()

	s.receiver.telemetryBuilder.YangReceiverConnectionsOpened.Add(ctx, 1)
	defer s.receiver.telemetryBuilder.YangReceiverConnectionsClosed.Add(ctx, 1)

	s.receiver.settings.Logger.Debug("New MDT dialout connection established")

	for {
		// Receive telemetry data from Cisco device
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			s.receiver.settings.Logger.Debug("MDT dialout connection closed by client")
			return nil
		}
		if err != nil {
			s.receiver.telemetryBuilder.YangReceiverGrpcErrors.Add(ctx, 1) // "receive_error", "stream_recv"
			s.receiver.settings.Logger.Error("Error receiving MDT data", zap.Error(err))
			return err
		}

		// Process the received telemetry data
		err = s.processTelemetryData(req)
		if err != nil {
			s.receiver.telemetryBuilder.YangReceiverGrpcErrors.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
				attribute.String("error_type", "processing_error"), attribute.String("error", "process_telemetry"),
			)))
			s.receiver.settings.Logger.Error("Error processing telemetry data",
				zap.Error(err), zap.Int64("req_id", req.ReqId))

			// Send error response back to device
			resp := &pb.MdtDialoutArgs{
				ReqId:  req.ReqId,
				Errors: fmt.Sprintf("Processing error: %v", err),
			}
			if sendErr := stream.Send(resp); sendErr != nil {
				s.receiver.telemetryBuilder.YangReceiverGrpcErrors.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
					attribute.String("error_type", "send_error"), attribute.String("error", "stream_send"),
				)))
				s.receiver.settings.Logger.Error("Failed to send error response", zap.Error(sendErr))
			}
			continue
		}

		// Send acknowledgment back to device
		resp := &pb.MdtDialoutArgs{
			ReqId: req.ReqId,
		}
		if err := stream.Send(resp); err != nil {
			s.receiver.settings.Logger.Error("Failed to send acknowledgment", zap.Error(err))
			return err
		}
	}
}

// processTelemetryData parses the incoming telemetry data and converts to OTEL metrics
func (s *grpcService) processTelemetryData(req *pb.MdtDialoutArgs) error {
	ctx := context.Background()
	startTime := time.Now()

	// Record message received metrics
	nodeID := "unknown"
	subscriptionID := fmt.Sprintf("%d", req.ReqId)

	if len(req.Data) == 0 {
		s.receiver.telemetryBuilder.YangReceiverMessagesDropped.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("node_id", nodeID),
			attribute.String("subscription_id", subscriptionID),
			attribute.String("reason", "empty_data"),
		)))
		return errors.New("empty telemetry data")
	}

	s.receiver.telemetryBuilder.YangReceiverMessagesReceived.Add(ctx, int64(len(req.Data)), metric.WithAttributeSet(attribute.NewSet(
		attribute.String("node_id", nodeID),
		attribute.String("subscription_id", subscriptionID),
	)))

	// Parse the telemetry message from the data field
	telemetryMsg := &pb.Telemetry{}
	if err := proto.Unmarshal(req.Data, telemetryMsg); err != nil {
		s.receiver.telemetryBuilder.YangReceiverMessagesDropped.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("node_id", nodeID),
			attribute.String("subscription_id", subscriptionID),
			attribute.String("reason", "unmarshal_error"),
		)))
		return fmt.Errorf("failed to unmarshal telemetry data: %w", err)
	}

	// Update nodeID with actual value
	if telemetryMsg.GetNodeIdStr() != "" {
		nodeID = telemetryMsg.GetNodeIdStr()
	}

	// Convert to OTEL metrics
	metrics := s.convertToOTELMetrics(telemetryMsg)
	if metrics.MetricCount() == 0 {
		s.receiver.telemetryBuilder.YangReceiverMessagesDropped.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("node_id", nodeID),
			attribute.String("subscription_id", subscriptionID),
			attribute.String("reason", "no_metrics_extracted"),
		)))
		s.receiver.settings.Logger.Warn("No metrics extracted from telemetry data",
			zap.String("encoding_path", telemetryMsg.EncodingPath),
			zap.String("node_id", telemetryMsg.GetNodeIdStr()))
		return nil
	}

	// Send metrics to the consumer
	err := s.receiver.consumer.ConsumeMetrics(ctx, metrics)
	if err != nil {
		s.receiver.telemetryBuilder.YangReceiverMessagesDropped.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("node_id", nodeID),
			attribute.String("subscription_id", subscriptionID),
			attribute.String("reason", "consumer_error"),
		)))
		return fmt.Errorf("failed to consume metrics: %w", err)
	}

	// Record successful processing with timing
	processingDuration := time.Since(startTime)
	yangModule := s.extractYANGModule(telemetryMsg.EncodingPath)
	s.receiver.telemetryBuilder.YangReceiverMessagesProcessed.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
		attribute.String("node_id", nodeID),
		attribute.String("subscription_id", subscriptionID),
		attribute.String("yang_module", yangModule),
	)))
	s.receiver.telemetryBuilder.YangReceiverProcessingDuration.Record(ctx, float64(processingDuration.Nanoseconds())/1000000)

	s.receiver.settings.Logger.Debug("Successfully processed telemetry data",
		zap.Int64("req_id", req.ReqId),
		zap.String("encoding_path", telemetryMsg.EncodingPath),
		zap.Int("metric_count", metrics.MetricCount()),
		zap.Duration("processing_duration", processingDuration))

	return nil
}

// extractYANGModule extracts the YANG module name from encoding path
func (*grpcService) extractYANGModule(encodingPath string) string {
	// Example: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics"
	if before, _, ok := strings.Cut(encodingPath, ":"); ok {
		return before
	}
	return "unknown"
}

// convertToOTELMetrics converts Cisco telemetry data to OpenTelemetry metrics format
func (s *grpcService) convertToOTELMetrics(telemetry *pb.Telemetry) pmetric.Metrics {
	// RFC YANG Parser analysis for this encoding path
	if s.rfcYangParser != nil {
		rfcAnalysis := s.rfcYangParser.AnalyzeTelemetryPath(telemetry.EncodingPath)
		if rfcAnalysis != nil && rfcAnalysis.IsValid {
			fmt.Printf("\n=== RFC YANG ANALYSIS ===\n")
			fmt.Printf("Encoding Path: %s\n", telemetry.EncodingPath)
			fmt.Printf("Module: %s\n", rfcAnalysis.ModuleName)
			fmt.Printf("XPath: %s\n", rfcAnalysis.XPath)
			fmt.Printf("List Path: %s\n", rfcAnalysis.ListPath)
			fmt.Printf("Semantic Type: %s\n", rfcAnalysis.SemanticType)
			fmt.Printf("List Keys: %v\n", rfcAnalysis.ListKeys)
			fmt.Printf("=========================\n\n")
		}
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes
	resource := resourceMetrics.Resource()
	resourceAttrs := resource.Attributes()

	if nodeID := telemetry.GetNodeIdStr(); nodeID != "" {
		resourceAttrs.PutStr("cisco.node_id", nodeID)
	}
	if subscriptionID := telemetry.GetSubscriptionIdStr(); subscriptionID != "" {
		resourceAttrs.PutStr("cisco.subscription_id", subscriptionID)
	}
	resourceAttrs.PutStr("cisco.encoding_path", telemetry.EncodingPath)

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scope := scopeMetrics.Scope()
	scope.SetName("cisco_telemetry_receiver")
	scope.SetVersion("1.0.0")

	// Process kvGPB data if present
	if len(telemetry.DataGpbkv) > 0 {
		s.processKvGPBData(scopeMetrics, telemetry)
	}

	// Process GPB table data if present
	if telemetry.DataGpb != nil {
		s.processGPBTableData(scopeMetrics, telemetry)
	}

	return metrics
}

// processKvGPBData processes key-value GPB formatted telemetry data
func (s *grpcService) processKvGPBData(scopeMetrics pmetric.ScopeMetrics, telemetry *pb.Telemetry) {
	timestamp := pcommon.Timestamp(telemetry.MsgTimestamp * 1000000) // Convert milliseconds to nanoseconds

	for _, field := range telemetry.DataGpbkv {
		s.processField(scopeMetrics, field, telemetry.EncodingPath, "", timestamp)
	}
} // processField recursively processes a telemetry field and creates metrics
func (s *grpcService) processField(scopeMetrics pmetric.ScopeMetrics, field *pb.TelemetryField, basePath, pathPrefix string, timestamp pcommon.Timestamp) {
	currentPath := pathPrefix
	if currentPath != "" {
		currentPath += "."
	}
	currentPath += field.Name

	// YANG analysis is now done within the metric creation methods
	// If field has a value, create a metric
	if field.ValueByType != nil {
		m := scopeMetrics.Metrics().AppendEmpty()

		// Get YANG data type information for this field
		yangDataType := s.yangParser.GetDataTypeForEncodingPath(basePath, field.Name)

		switch value := field.ValueByType.(type) {
		case *pb.TelemetryField_Uint32Value:
			s.createYANGAwareMetric(m, currentPath, basePath, float64(value.Uint32Value), timestamp, yangDataType)
		case *pb.TelemetryField_Uint64Value:
			s.createYANGAwareMetric(m, currentPath, basePath, float64(value.Uint64Value), timestamp, yangDataType)
		case *pb.TelemetryField_Sint32Value:
			s.createYANGAwareMetric(m, currentPath, basePath, float64(value.Sint32Value), timestamp, yangDataType)
		case *pb.TelemetryField_Sint64Value:
			s.createYANGAwareMetric(m, currentPath, basePath, float64(value.Sint64Value), timestamp, yangDataType)
		case *pb.TelemetryField_DoubleValue:
			s.createYANGAwareMetric(m, currentPath, basePath, value.DoubleValue, timestamp, yangDataType)
		case *pb.TelemetryField_FloatValue:
			s.createYANGAwareMetric(m, currentPath, basePath, float64(value.FloatValue), timestamp, yangDataType)
		case *pb.TelemetryField_BoolValue:
			val := 0.0
			if value.BoolValue {
				val = 1.0
			}
			s.createYANGAwareMetric(m, currentPath, basePath, val, timestamp, yangDataType)
		case *pb.TelemetryField_StringValue:
			// For string values, create YANG-aware info m
			s.createYANGAwareInfoMetric(m, currentPath, basePath, value.StringValue, timestamp, yangDataType)
		default:
			// Remove the metric we added if we can't handle the type
			scopeMetrics.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				return m.Name() == ""
			})
		}
	}

	// Process nested fields recursively
	for _, nestedField := range field.Fields {
		s.processField(scopeMetrics, nestedField, basePath, currentPath, timestamp)
	}
}

// processGPBTableData processes GPB table formatted telemetry data
func (s *grpcService) processGPBTableData(_ pmetric.ScopeMetrics, telemetry *pb.Telemetry) {
	// For GPB table data, we would need specific protobuf definitions for each encoding path
	// This is a placeholder implementation
	s.receiver.settings.Logger.Debug("GPB table data processing not implemented",
		zap.String("encoding_path", telemetry.EncodingPath))
}

// isKeyField checks if a field name is a key field based on YANG analysis
func (*grpcService) isKeyField(fieldName string, analysis *internal.PathAnalysis) bool {
	if analysis == nil {
		return false
	}

	// Check if this field matches any known key fields for the path
	for _, keyField := range analysis.Keys {
		if fieldName == keyField {
			return true
		}
	}

	// Additional heuristics for common key patterns
	commonKeys := []string{"name", "id", "index", "interface-name", "neighbor-id", "router-id"}
	return slices.Contains(commonKeys, fieldName)
}

// extractFieldName extracts the field name from a metric name path
func (*grpcService) extractFieldName(metricName string) string {
	// Remove common prefixes and get the last component
	parts := strings.Split(metricName, ".")
	if len(parts) == 0 {
		return metricName
	}

	// Get the last part, which is usually the field name
	fieldName := parts[len(parts)-1]

	// Remove common suffixes like "_info"
	fieldName = strings.TrimSuffix(fieldName, "_info")

	return fieldName
}

// createYANGAwareMetric creates a metric with YANG data type awareness
func (s *grpcService) createYANGAwareMetric(metric pmetric.Metric, name, encodingPath string, value float64, timestamp pcommon.Timestamp, yangDataType *internal.YANGDataType) {
	// Determine metric name and type based on YANG information
	metricName := fmt.Sprintf("cisco.%s", name)
	metricDescription := fmt.Sprintf("Cisco telemetry metric from %s", encodingPath)
	metricUnit := "1" // Default unit

	if yangDataType != nil {
		// Use YANG-provided description and units
		if yangDataType.Description != "" {
			metricDescription = yangDataType.Description
		}
		if yangDataType.Units != "" {
			metricUnit = yangDataType.Units
		}

		// Add YANG type to metric name for clarity
		if yangDataType.Type != "" {
			metricName = fmt.Sprintf("cisco.%s", name)
		}
	}

	metric.SetName(metricName)
	metric.SetDescription(metricDescription)
	metric.SetUnit(metricUnit)

	// Determine if this should be a counter or gauge based on YANG data type
	if yangDataType != nil && yangDataType.IsCounterType() {
		// Create a sum metric for counters (monotonic increasing)
		sum := metric.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		dp := sum.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.SetTimestamp(timestamp)

		// Add attributes
		s.addYANGAttributes(dp.Attributes(), encodingPath, yangDataType, name)
	} else {
		// Create a gauge metric for everything else
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.SetTimestamp(timestamp)

		// Add attributes
		s.addYANGAttributes(dp.Attributes(), encodingPath, yangDataType, name)
	}
}

// createYANGAwareInfoMetric creates an info metric with YANG data type awareness
func (s *grpcService) createYANGAwareInfoMetric(metric pmetric.Metric, name, encodingPath, value string, timestamp pcommon.Timestamp, yangDataType *internal.YANGDataType) {
	metricName := fmt.Sprintf("cisco.%s_info", name)
	metricDescription := fmt.Sprintf("Cisco telemetry info metric from %s", encodingPath)

	if yangDataType != nil && yangDataType.Description != "" {
		metricDescription = yangDataType.Description
	}

	metric.SetName(metricName)
	metric.SetDescription(metricDescription)
	metric.SetUnit("1")

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0) // Info metrics always have value 1
	dp.SetTimestamp(timestamp)

	// Add the string value as an attribute
	dp.Attributes().PutStr("value", value)

	// Add YANG attributes
	s.addYANGAttributes(dp.Attributes(), encodingPath, yangDataType, name)
}

// addYANGAttributes adds YANG-derived attributes to metric data points
func (s *grpcService) addYANGAttributes(attrs pcommon.Map, encodingPath string, yangDataType *internal.YANGDataType, fieldName string) {
	// Always add encoding path
	attrs.PutStr("encoding_path", encodingPath)

	// Use RFC-compliant YANG parser for enhanced analysis
	rfcAnalysis := s.rfcYangParser.AnalyzeTelemetryPath(encodingPath)
	if rfcAnalysis != nil && rfcAnalysis.IsValid {
		if rfcAnalysis.ModuleName != "" {
			attrs.PutStr("yang.module", rfcAnalysis.ModuleName)
		}
		if rfcAnalysis.ListPath != "" {
			attrs.PutStr("yang.list_path", rfcAnalysis.ListPath)
		}
		if rfcAnalysis.SemanticType != "" {
			attrs.PutStr("yang.semantic_type", rfcAnalysis.SemanticType)
		}

		// Add list key information
		if len(rfcAnalysis.ListKeys) > 0 {
			// Check if current field is a key field
			for _, key := range rfcAnalysis.ListKeys {
				if strings.Contains(fieldName, key) {
					attrs.PutStr("yang.is_key", "true")
					attrs.PutStr("yang.key_type", key)
					break
				}
			}
		}

		// Add inferred data type from RFC parser
		if len(rfcAnalysis.PathSegments) > 0 {
			leafName := rfcAnalysis.PathSegments[len(rfcAnalysis.PathSegments)-1]
			inferredType := s.rfcYangParser.InferDataTypeFromPath(leafName)
			if inferredType != nil && inferredType.Name != "" {
				attrs.PutStr("yang.data_type", inferredType.Name)
			}
		}
	}

	// Fallback to basic YANG parser analysis if RFC parser fails
	if rfcAnalysis == nil || !rfcAnalysis.IsValid {
		analysis := s.yangParser.AnalyzeEncodingPath(encodingPath)
		if analysis != nil {
			if analysis.ModuleName != "" {
				attrs.PutStr("yang.module", analysis.ModuleName)
			}
			if analysis.ListPath != "" {
				attrs.PutStr("yang.list_path", analysis.ListPath)
			}

			// Check if this is a key field
			if s.isKeyField(fieldName, analysis) {
				attrs.PutStr("yang.is_key", "true")
				attrs.PutStr("yang.key_type", fieldName)
			}
		}
	}

	// Add YANG data type information from basic parser
	if yangDataType != nil {
		if yangDataType.Type != "" {
			attrs.PutStr("yang.data_type", yangDataType.Type)
		}
		if yangDataType.Units != "" {
			attrs.PutStr("yang.units", yangDataType.Units)
		}
		if yangDataType.Description != "" {
			attrs.PutStr("yang.description", yangDataType.Description)
		}

		// Add semantic information
		if yangDataType.IsCounterType() {
			attrs.PutStr("yang.semantic_type", "counter")
		} else if yangDataType.IsGaugeType() {
			attrs.PutStr("yang.semantic_type", "gauge")
		}

		// Add range information if available
		if yangDataType.Range != nil {
			if yangDataType.Range.Min != nil {
				attrs.PutInt("yang.min_value", *yangDataType.Range.Min)
			}
			if yangDataType.Range.Max != nil {
				attrs.PutInt("yang.max_value", *yangDataType.Range.Max)
			}
		}
	}
}
