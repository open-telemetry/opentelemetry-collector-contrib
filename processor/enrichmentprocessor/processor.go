// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// enrichmentProcessor is the main processor implementation
type enrichmentProcessor struct {
	config      *Config
	logger      *zap.Logger
	dataSources map[string]DataSource
}

// newEnrichmentProcessor creates a new enrichment processor
func newEnrichmentProcessor(ctx context.Context, set processor.Settings, config *Config) (*enrichmentProcessor, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	processor := &enrichmentProcessor{
		config:      config,
		logger:      set.Logger,
		dataSources: make(map[string]DataSource),
	}

	// If config is empty (default config case), create a no-op processor
	if len(config.DataSources) == 0 && len(config.EnrichmentRules) == 0 {
		set.Logger.Debug("Creating enrichment processor with empty configuration - no enrichment will be performed")
		return processor, nil
	}

	// Initialize data sources
	for _, dsConfig := range config.DataSources {
		dataSource, err := processor.createDataSource(dsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create data source %s: %w", dsConfig.Name, err)
		}

		processor.dataSources[dsConfig.Name] = dataSource

		// Start the data source
		if err := dataSource.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start data source %s: %w", dsConfig.Name, err)
		}
	}

	return processor, nil
}

// createDataSource creates a data source based on configuration
func (ep *enrichmentProcessor) createDataSource(config DataSourceConfig) (DataSource, error) {
	// Extract index fields from enrichment rules that use this data source
	var indexFields []string
	for _, rule := range ep.config.EnrichmentRules {
		if rule.DataSource == config.Name && rule.LookupField != "" {
			// Check if this field is already in the list to avoid duplicates
			found := false
			for _, existing := range indexFields {
				if existing == rule.LookupField {
					found = true
					break
				}
			}
			if !found {
				indexFields = append(indexFields, rule.LookupField)
			}
		}
	}

	switch config.Type {
	case "http":
		if config.HTTP == nil {
			return nil, errors.New("HTTP configuration is required for http data source")
		}
		return newHTTPDataSource(*config.HTTP, ep.logger, indexFields), nil

	case "file":
		if config.File == nil {
			return nil, errors.New("File configuration is required for file data source")
		}
		return newFileDataSource(*config.File, ep.logger, indexFields), nil

	default:
		return nil, errors.New("unsupported data source type: " + config.Type)
	}
}

// Start starts the processor
func (*enrichmentProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown shuts down the processor
func (ep *enrichmentProcessor) Shutdown(_ context.Context) error {
	for _, dataSource := range ep.dataSources {
		if err := dataSource.Stop(); err != nil {
			ep.logger.Error("Failed to stop enrichment data source", zap.Error(err))
		}
	}
	return nil
}

// enrichAttributes enriches attributes based on enrichment rules
func (ep *enrichmentProcessor) enrichAttributes(attributes pcommon.Map, enrichContext string) {
	// Early return if no enrichment rules configured
	if len(ep.config.EnrichmentRules) == 0 {
		ep.logger.Warn("No enrichment rules configured for processor")
		return
	}

	for _, rule := range ep.config.EnrichmentRules {
		if rule.Context != enrichContext {
			continue
		}
		if err := ep.applyEnrichmentRule(rule, attributes); err != nil {
			ep.logger.Debug("Failed to apply enrichment rule, continuing with next rule",
				zap.String("rule_name", rule.Name),
				zap.String("lookup_field", rule.LookupField),
				zap.Error(err))
			// Continue with other rules even if one fails
		}
	}
}

// applyEnrichmentRule applies a single enrichment rule
func (ep *enrichmentProcessor) applyEnrichmentRule(rule EnrichmentRule, attributes pcommon.Map) error {
	// Get lookup key value
	lookupValue, exists := attributes.Get(rule.LookupAttributeKey)
	if !exists {
		return errors.New("lookup key " + rule.LookupAttributeKey + " not found in attributes")
	}

	lookupKey := lookupValue.AsString()
	if lookupKey == "" {
		return errors.New("lookup key " + rule.LookupAttributeKey + " has empty value")
	}

	// Get data source
	dataSource, exists := ep.dataSources[rule.DataSource]
	if !exists {
		return errors.New("data source " + rule.DataSource + " not found")
	}

	// Perform lookup
	enrichmentRow, index, err := dataSource.Lookup(rule.LookupField, lookupKey)
	if err != nil {
		return fmt.Errorf("lookup failed: %w", err)
	}

	// Apply mappings
	ep.applyMappings(rule.Mappings, enrichmentRow, index, attributes)

	return nil
}

// applyMappings applies field mappings to attributes
func (ep *enrichmentProcessor) applyMappings(mappings []FieldMapping, enrichmentRow []string, index map[string]int, attributes pcommon.Map) {
	for _, mapping := range mappings {
		// Find the column index for the source field
		columnIndex, exists := index[mapping.SourceField]
		if !exists {
			ep.logger.Warn("Enrichment source field not found in data",
				zap.String("source_field", mapping.SourceField),
				zap.String("target_attribute", mapping.TargetAttribute))
			continue
		}

		// Check if the column index is valid for this row
		if columnIndex >= len(enrichmentRow) {
			ep.logger.Warn("Enrichment column index out of range",
				zap.String("source_field", mapping.SourceField),
				zap.Int("column_index", columnIndex),
				zap.Int("row_length", len(enrichmentRow)))
			continue
		}

		value := enrichmentRow[columnIndex]
		if value != "" { // Only apply if value is not empty
			// Set the attribute directly
			attributes.PutStr(mapping.TargetAttribute, value)
		}
	}
}

// Traces processor implementation
type tracesProcessor struct {
	*enrichmentProcessor
	nextConsumer consumer.Traces
}

func newTracesProcessor(ctx context.Context, set processor.Settings, config *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	ep, err := newEnrichmentProcessor(ctx, set, config)
	if err != nil {
		return nil, err
	}

	return &tracesProcessor{
		enrichmentProcessor: ep,
		nextConsumer:        nextConsumer,
	}, nil
}

func (tp *tracesProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		tp.enrichAttributes(rs.Resource().Attributes(), ENRICHCONTEXTRESOURCE)

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				tp.enrichAttributes(span.Attributes(), ENRICHCONTEXTINDIVIDUAL)
			}
		}
	}

	return tp.nextConsumer.ConsumeTraces(ctx, td)
}

func (*tracesProcessor) Capabilities() consumer.Capabilities {
	return processorCapabilities
}

func (tp *tracesProcessor) Start(ctx context.Context, host component.Host) error {
	return tp.enrichmentProcessor.Start(ctx, host)
}

func (tp *tracesProcessor) Shutdown(ctx context.Context) error {
	return tp.enrichmentProcessor.Shutdown(ctx)
}

// Metrics processor implementation
type metricsProcessor struct {
	*enrichmentProcessor
	nextConsumer consumer.Metrics
}

func newMetricsProcessor(ctx context.Context, set processor.Settings, config *Config, nextConsumer consumer.Metrics) (processor.Metrics, error) {
	ep, err := newEnrichmentProcessor(ctx, set, config)
	if err != nil {
		return nil, err
	}

	return &metricsProcessor{
		enrichmentProcessor: ep,
		nextConsumer:        nextConsumer,
	}, nil
}

func (mp *metricsProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		mp.enrichAttributes(rm.Resource().Attributes(), ENRICHCONTEXTRESOURCE)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				mp.enrichMetricDataPoints(metric)
			}
		}
	}

	return mp.nextConsumer.ConsumeMetrics(ctx, md)
}

func (mp *metricsProcessor) enrichMetricDataPoints(metric pmetric.Metric) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			mp.enrichAttributes(dp.Attributes(), ENRICHCONTEXTINDIVIDUAL)
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			mp.enrichAttributes(dp.Attributes(), ENRICHCONTEXTINDIVIDUAL)
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			mp.enrichAttributes(dp.Attributes(), ENRICHCONTEXTINDIVIDUAL)
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			mp.enrichAttributes(dp.Attributes(), ENRICHCONTEXTINDIVIDUAL)
		}
	}
}

func (*metricsProcessor) Capabilities() consumer.Capabilities {
	return processorCapabilities
}

func (mp *metricsProcessor) Start(ctx context.Context, host component.Host) error {
	return mp.enrichmentProcessor.Start(ctx, host)
}

func (mp *metricsProcessor) Shutdown(ctx context.Context) error {
	return mp.enrichmentProcessor.Shutdown(ctx)
}

// Logs processor implementation
type logsProcessor struct {
	*enrichmentProcessor
	nextConsumer consumer.Logs
}

func newLogsProcessor(ctx context.Context, set processor.Settings, config *Config, nextConsumer consumer.Logs) (processor.Logs, error) {
	ep, err := newEnrichmentProcessor(ctx, set, config)
	if err != nil {
		return nil, err
	}

	return &logsProcessor{
		enrichmentProcessor: ep,
		nextConsumer:        nextConsumer,
	}, nil
}

func (lp *logsProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		lp.enrichAttributes(rl.Resource().Attributes(), ENRICHCONTEXTRESOURCE)

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				logRecord := sl.LogRecords().At(k)
				lp.enrichAttributes(logRecord.Attributes(), ENRICHCONTEXTINDIVIDUAL)
			}
		}
	}

	return lp.nextConsumer.ConsumeLogs(ctx, ld)
}

func (*logsProcessor) Capabilities() consumer.Capabilities {
	return processorCapabilities
}

func (lp *logsProcessor) Start(ctx context.Context, host component.Host) error {
	return lp.enrichmentProcessor.Start(ctx, host)
}

func (lp *logsProcessor) Shutdown(ctx context.Context) error {
	return lp.enrichmentProcessor.Shutdown(ctx)
}
