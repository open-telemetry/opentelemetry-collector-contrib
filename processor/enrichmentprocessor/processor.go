// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

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
	config       *Config
	logger       *zap.Logger
	dataSources  map[string]DataSource
	cache        *Cache
	nextConsumer interface{}
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
		cache:       NewCache(config.Cache),
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

	// Start cache cleanup routine
	go processor.cacheCleanupRoutine(ctx)

	return processor, nil
}

// createDataSource creates a data source based on configuration
func (ep *enrichmentProcessor) createDataSource(config DataSourceConfig) (DataSource, error) {
	switch config.Type {
	case "http":
		if config.HTTP == nil {
			return nil, fmt.Errorf("HTTP configuration is required for http data source")
		}
		return NewHTTPDataSource(*config.HTTP, ep.logger), nil

	case "file":
		if config.File == nil {
			return nil, fmt.Errorf("File configuration is required for file data source")
		}
		return NewFileDataSource(*config.File, ep.logger), nil

	case "prometheus":
		if config.Prometheus == nil {
			return nil, fmt.Errorf("Prometheus configuration is required for prometheus data source")
		}
		// TODO: Implement Prometheus data source
		return nil, fmt.Errorf("prometheus data source not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported data source type: %s", config.Type)
	}
}

// cacheCleanupRoutine periodically cleans up expired cache entries
func (ep *enrichmentProcessor) cacheCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ep.cache.Cleanup()
		}
	}
}

// Start starts the processor
func (ep *enrichmentProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

// Shutdown shuts down the processor
func (ep *enrichmentProcessor) Shutdown(ctx context.Context) error {
	for _, dataSource := range ep.dataSources {
		if err := dataSource.Stop(); err != nil {
			ep.logger.Error("Failed to stop data source", zap.Error(err))
		}
	}
	return nil
}

// enrichAttributes enriches attributes based on enrichment rules
func (ep *enrichmentProcessor) enrichAttributes(ctx context.Context, attributes pcommon.Map) error {
	for _, rule := range ep.config.EnrichmentRules {
		if err := ep.applyEnrichmentRule(ctx, rule, attributes); err != nil {
			ep.logger.Debug("Failed to apply enrichment rule",
				zap.String("rule", rule.Name),
				zap.Error(err))
			// Continue with other rules even if one fails
		}
	}
	return nil
}

// applyEnrichmentRule applies a single enrichment rule
func (ep *enrichmentProcessor) applyEnrichmentRule(ctx context.Context, rule EnrichmentRule, attributes pcommon.Map) error {
	// Check conditions first
	if !ep.evaluateConditions(rule.Conditions, attributes) {
		return nil // Conditions not met, skip this rule
	}

	// Get lookup key value
	lookupValue, exists := attributes.Get(rule.LookupKey)
	if !exists {
		return fmt.Errorf("lookup key %s not found in attributes", rule.LookupKey)
	}

	lookupKey := lookupValue.AsString()
	if lookupKey == "" {
		return fmt.Errorf("lookup key %s has empty value", rule.LookupKey)
	}

	// Check cache first
	if enrichmentData, found := ep.cache.Get(lookupKey); found {
		ep.applyMappings(rule.Mappings, enrichmentData, attributes)
		return nil
	}

	// Get data source
	dataSource, exists := ep.dataSources[rule.DataSource]
	if !exists {
		return fmt.Errorf("data source %s not found", rule.DataSource)
	}

	// Perform lookup
	enrichmentData, err := dataSource.Lookup(ctx, lookupKey)
	if err != nil {
		return fmt.Errorf("lookup failed: %w", err)
	}

	// Cache the result
	ep.cache.Set(lookupKey, enrichmentData)

	// Apply mappings
	ep.applyMappings(rule.Mappings, enrichmentData, attributes)

	return nil
}

// evaluateConditions checks if all conditions are met
func (ep *enrichmentProcessor) evaluateConditions(conditions []Condition, attributes pcommon.Map) bool {
	for _, condition := range conditions {
		if !ep.evaluateCondition(condition, attributes) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (ep *enrichmentProcessor) evaluateCondition(condition Condition, attributes pcommon.Map) bool {
	attributeValue, exists := attributes.Get(condition.Attribute)
	if !exists {
		return false
	}

	attrStr := attributeValue.AsString()

	switch condition.Operator {
	case "equals":
		return attrStr == condition.Value
	case "contains":
		return strings.Contains(attrStr, condition.Value)
	case "regex":
		if matched, err := regexp.MatchString(condition.Value, attrStr); err == nil {
			return matched
		}
		return false
	case "not_equals":
		return attrStr != condition.Value
	default:
		ep.logger.Warn("Unknown condition operator", zap.String("operator", condition.Operator))
		return false
	}
}

// applyMappings applies field mappings to attributes
func (ep *enrichmentProcessor) applyMappings(mappings []FieldMapping, enrichmentData map[string]interface{}, attributes pcommon.Map) {
	for _, mapping := range mappings {
		if value, exists := enrichmentData[mapping.SourceField]; exists {
			// Apply transformation if specified
			transformedValue := ep.applyTransform(mapping.Transform, value)

			// Set the attribute
			if strValue, ok := transformedValue.(string); ok {
				attributes.PutStr(mapping.TargetAttribute, strValue)
			} else {
				// Convert to string for non-string values
				attributes.PutStr(mapping.TargetAttribute, fmt.Sprintf("%v", transformedValue))
			}
		}
	}
}

// applyTransform applies a transformation function to a value
func (ep *enrichmentProcessor) applyTransform(transform string, value interface{}) interface{} {
	if transform == "" {
		return value
	}

	strValue := fmt.Sprintf("%v", value)

	switch transform {
	case "upper":
		return strings.ToUpper(strValue)
	case "lower":
		return strings.ToLower(strValue)
	case "trim":
		return strings.TrimSpace(strValue)
	default:
		ep.logger.Warn("Unknown transform function", zap.String("transform", transform))
		return value
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
		if err := tp.enrichAttributes(ctx, rs.Resource().Attributes()); err != nil {
			tp.logger.Error("Failed to enrich resource attributes", zap.Error(err))
		}

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				if err := tp.enrichAttributes(ctx, span.Attributes()); err != nil {
					tp.logger.Error("Failed to enrich span attributes", zap.Error(err))
				}
			}
		}
	}

	return tp.nextConsumer.ConsumeTraces(ctx, td)
}

func (tp *tracesProcessor) Capabilities() consumer.Capabilities {
	return processorCapabilities
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
		if err := mp.enrichAttributes(ctx, rm.Resource().Attributes()); err != nil {
			mp.logger.Error("Failed to enrich resource attributes", zap.Error(err))
		}

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				mp.enrichMetricDataPoints(ctx, metric)
			}
		}
	}

	return mp.nextConsumer.ConsumeMetrics(ctx, md)
}

func (mp *metricsProcessor) enrichMetricDataPoints(ctx context.Context, metric pmetric.Metric) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if err := mp.enrichAttributes(ctx, dp.Attributes()); err != nil {
				mp.logger.Error("Failed to enrich gauge data point attributes", zap.Error(err))
			}
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if err := mp.enrichAttributes(ctx, dp.Attributes()); err != nil {
				mp.logger.Error("Failed to enrich sum data point attributes", zap.Error(err))
			}
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if err := mp.enrichAttributes(ctx, dp.Attributes()); err != nil {
				mp.logger.Error("Failed to enrich histogram data point attributes", zap.Error(err))
			}
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if err := mp.enrichAttributes(ctx, dp.Attributes()); err != nil {
				mp.logger.Error("Failed to enrich summary data point attributes", zap.Error(err))
			}
		}
	}
}

func (mp *metricsProcessor) Capabilities() consumer.Capabilities {
	return processorCapabilities
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
		if err := lp.enrichAttributes(ctx, rl.Resource().Attributes()); err != nil {
			lp.logger.Error("Failed to enrich resource attributes", zap.Error(err))
		}

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				logRecord := sl.LogRecords().At(k)
				if err := lp.enrichAttributes(ctx, logRecord.Attributes()); err != nil {
					lp.logger.Error("Failed to enrich log record attributes", zap.Error(err))
				}
			}
		}
	}

	return lp.nextConsumer.ConsumeLogs(ctx, ld)
}

func (lp *logsProcessor) Capabilities() consumer.Capabilities {
	return processorCapabilities
}
