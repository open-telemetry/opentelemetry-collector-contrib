// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsfilterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampmetricsfilterprocessor"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/prometheus/promql/parser"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// AlertRule represents a single alert rule
type AlertRule struct {
	Name              string `yaml:"name"`
	Interval          string `yaml:"interval"`
	Expr              string `yaml:"expr"`
	IsAvailability    bool   `yaml:"isAvailability"`
	WarnOperator      string `yaml:"warnOperator,omitempty"`
	WarnThreshold     string `yaml:"warnThreshold,omitempty"`
	CriticalOperator  string `yaml:"criticalOperator,omitempty"`
	CriticalThreshold string `yaml:"criticalThreshold,omitempty"`
	AlertSub          string `yaml:"alertSub,omitempty"`
	AlertBody         string `yaml:"alertBody,omitempty"`
}

// AlertDefinition represents a single alert definition group
type AlertDefinition struct {
	ResourceType string      `yaml:"resourceType"`
	Rules        []AlertRule `yaml:"rules"`
}

// AlertDefinitions represents the structure of alert definitions
type AlertDefinitions struct {
	AlertDefinitions []AlertDefinition `yaml:"alertDefinitions"`
}

// filterProcessor implements the alert metrics extractor processor
type filterProcessor struct {
	config       *Config
	nextConsumer consumer.Metrics
	logger       *zap.Logger
	client       kubernetes.Interface

	// Thread-safe map of metric names to extract
	metricsMutex sync.RWMutex
	metricsMap   map[string]bool

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// Ensure filterProcessor implements processor.Metrics interface
var _ processor.Metrics = (*filterProcessor)(nil)

// newFilterProcessor creates a new instance of the filterProcessor
func newFilterProcessor(settings processor.Settings, config *Config, nextConsumer consumer.Metrics) (*filterProcessor, error) {
	// Create Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
	}

	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fp := &filterProcessor{
		config:       config,
		nextConsumer: nextConsumer,
		logger:       settings.Logger,
		client:       client,
		metricsMap:   make(map[string]bool),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Initial load of alert definitions
	if err := fp.loadAlertDefinitions(); err != nil {
		fp.logger.Error("Failed to load initial alert definitions", zap.Error(err))
		// Don't return error here, just log it and continue
	}

	// Start watching for ConfigMap changes
	go fp.watchConfigMap()

	return fp, nil
}

// Start starts the processor
func (fp *filterProcessor) Start(ctx context.Context, host component.Host) error {
	fp.logger.Info("Starting alert metrics extractor processor",
		zap.String("configmap_name", fp.config.AlertConfigMapName),
		zap.String("configmap_key", fp.config.AlertConfigMapKey),
		zap.String("namespace", fp.config.Namespace))

	// Log the current state of the metrics map
	fp.metricsMutex.RLock()
	currentMetricsCount := len(fp.metricsMap)
	fp.metricsMutex.RUnlock()

	fp.logger.Info("Processor started with metrics configuration",
		zap.Int("metrics_count", currentMetricsCount))

	return nil
}

// Shutdown stops the processor
func (fp *filterProcessor) Shutdown(ctx context.Context) error {
	fp.logger.Info("Shutting down alert metrics extractor processor")
	fp.cancel()
	return nil
}

// Capabilities returns the consumer capabilities
func (fp *filterProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeMetrics processes the metrics
func (fp *filterProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	fp.metricsMutex.RLock()
	defer fp.metricsMutex.RUnlock()

	// Count incoming metrics
	totalIncomingMetrics := 0
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			totalIncomingMetrics += sm.Metrics().Len()
		}
	}

	// Early return if no metrics to process
	if totalIncomingMetrics == 0 {
		fp.logger.Debug("No incoming metrics to process")
		return fp.nextConsumer.ConsumeMetrics(ctx, md)
	}

	// If no metrics are configured for filtering, drop all metrics
	if len(fp.metricsMap) == 0 {
		fp.logger.Info("No metrics filter configured, dropping all metrics",
			zap.Int("dropped_metrics", totalIncomingMetrics))
		// Return empty metrics to effectively drop all metrics
		return fp.nextConsumer.ConsumeMetrics(ctx, pmetric.NewMetrics())
	}

	// Create a new metrics object to hold filtered metrics
	filteredMetrics := pmetric.NewMetrics()
	totalFilteredMetrics := 0

	// Iterate through all resource metrics
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		// Create a new resource metrics container only if we find matching metrics
		var filteredRM pmetric.ResourceMetrics
		resourceHasMatchingMetrics := false

		// Iterate through all scope metrics
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			// Create a new scope metrics container only if we find matching metrics
			var filteredSM pmetric.ScopeMetrics
			scopeHasMatchingMetrics := false

			// Iterate through all metrics
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				metricName := metric.Name()
				// Convert dots to underscores for Prometheus compatibility
				metricName = strings.ReplaceAll(metricName, ".", "_")

				// Check if this metric should be included
				if fp.metricsMap[metricName] {
					// Create containers only when we have a matching metric
					if !resourceHasMatchingMetrics {
						filteredRM = filteredMetrics.ResourceMetrics().AppendEmpty()
						rm.Resource().CopyTo(filteredRM.Resource())
						resourceHasMatchingMetrics = true
					}

					if !scopeHasMatchingMetrics {
						filteredSM = filteredRM.ScopeMetrics().AppendEmpty()
						sm.Scope().CopyTo(filteredSM.Scope())
						scopeHasMatchingMetrics = true
					}

					filteredMetric := filteredSM.Metrics().AppendEmpty()
					metric.CopyTo(filteredMetric)
					totalFilteredMetrics++
				}
			}
		}
	}

	fp.logger.Info("Metrics filtering completed",
		zap.Int("incoming_metrics", totalIncomingMetrics),
		zap.Int("filtered_metrics", totalFilteredMetrics),
		zap.Int("configured_filter_count", len(fp.metricsMap)))

	if totalFilteredMetrics == 0 {
		fp.logger.Warn("No metrics passed through the filter - check configuration")
	}

	return fp.nextConsumer.ConsumeMetrics(ctx, filteredMetrics)
}

// loadAlertDefinitions loads alert definitions from the ConfigMap
func (fp *filterProcessor) loadAlertDefinitions() error {
	fp.logger.Info("Loading alert definitions from ConfigMap",
		zap.String("configmap", fp.config.AlertConfigMapName),
		zap.String("namespace", fp.config.Namespace),
		zap.String("key", fp.config.AlertConfigMapKey))

	configMap, err := fp.client.CoreV1().ConfigMaps(fp.config.Namespace).Get(
		context.TODO(),
		fp.config.AlertConfigMapName,
		metav1.GetOptions{},
	)
	if err != nil {
		fp.logger.Error("Failed to get ConfigMap",
			zap.String("configmap", fp.config.AlertConfigMapName),
			zap.String("namespace", fp.config.Namespace),
			zap.Error(err))
		return fmt.Errorf("failed to get ConfigMap %s/%s: %w", fp.config.Namespace, fp.config.AlertConfigMapName, err)
	}

	alertDefData, exists := configMap.Data[fp.config.AlertConfigMapKey]
	if !exists {
		fp.logger.Error("Alert definitions key not found in ConfigMap",
			zap.String("key", fp.config.AlertConfigMapKey),
			zap.Strings("available_keys", getKeys(configMap.Data)))
		return fmt.Errorf("key %s not found in ConfigMap %s/%s", fp.config.AlertConfigMapKey, fp.config.Namespace, fp.config.AlertConfigMapName)
	}

	var alertDefs AlertDefinitions
	if err := yaml.Unmarshal([]byte(alertDefData), &alertDefs); err != nil {
		fp.logger.Error("Failed to unmarshal alert definitions", zap.Error(err))
		return fmt.Errorf("failed to unmarshal alert definitions: %w", err)
	}

	// Extract metrics from alert expressions
	newMetricsMap := make(map[string]bool)
	for _, alertDef := range alertDefs.AlertDefinitions {
		for _, rule := range alertDef.Rules {
			metrics := fp.extractMetricsFromExpression(rule.Expr)
			for _, metric := range metrics {
				newMetricsMap[metric] = true
			}
		}
	}

	// Update the global metrics map
	fp.metricsMutex.Lock()
	fp.metricsMap = newMetricsMap
	fp.metricsMutex.Unlock()

	fp.logger.Info("Successfully loaded alert definitions",
		zap.Int("alert_definitions", len(alertDefs.AlertDefinitions)),
		zap.Int("unique_metrics", len(newMetricsMap)))

	return nil
}

// extractMetricNames recursively walks the AST to extract metric names
func extractMetricNames(node parser.Node, metrics map[string]struct{}) {
	switch n := node.(type) {
	case *parser.VectorSelector:
		metrics[n.Name] = struct{}{}
	case *parser.MatrixSelector:
		if vs, ok := n.VectorSelector.(*parser.VectorSelector); ok {
			metrics[vs.Name] = struct{}{}
		}
	}

	for _, child := range parser.Children(node) {
		extractMetricNames(child, metrics)
	}
}

// extractMetricsFromExpression extracts metric names from a PromQL expression
func (fp *filterProcessor) extractMetricsFromExpression(expr string) []string {
	parsedExpr, err := parser.ParseExpr(expr)
	if err != nil {
		fp.logger.Warn("Failed to parse PromQL expression", zap.String("expr", expr), zap.Error(err))
		return nil
	}

	metrics := make(map[string]struct{})
	extractMetricNames(parsedExpr, metrics)

	var result []string
	for metric := range metrics {
		result = append(result, metric)
	}

	return result
}

// watchConfigMap watches for changes to the ConfigMap
func (fp *filterProcessor) watchConfigMap() {
	for {
		select {
		case <-fp.ctx.Done():
			return
		default:
			fp.doWatch()
		}
	}
}

// doWatch performs the actual watching with retry logic
func (fp *filterProcessor) doWatch() {
	defer func() {
		// Wait before retrying
		select {
		case <-fp.ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}()

	watcher, err := fp.client.CoreV1().ConfigMaps(fp.config.Namespace).Watch(
		context.TODO(),
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", fp.config.AlertConfigMapName),
		},
	)
	if err != nil {
		fp.logger.Error("Failed to create ConfigMap watcher", zap.Error(err))
		return
	}
	defer watcher.Stop()

	fp.logger.Info("Started watching ConfigMap for changes",
		zap.String("configmap", fp.config.AlertConfigMapName),
		zap.String("namespace", fp.config.Namespace))

	for {
		select {
		case <-fp.ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				fp.logger.Warn("ConfigMap watcher channel closed, will retry")
				return
			}

			switch event.Type {
			case watch.Modified, watch.Added:
				fp.logger.Info("ConfigMap changed, reloading alert definitions")
				if err := fp.loadAlertDefinitions(); err != nil {
					fp.logger.Error("Failed to reload alert definitions", zap.Error(err))
				}
			case watch.Deleted:
				fp.logger.Warn("ConfigMap was deleted, clearing metrics map")
				fp.metricsMutex.Lock()
				fp.metricsMap = make(map[string]bool)
				fp.metricsMutex.Unlock()
			}
		}
	}
}

// getKeys returns a slice of keys from a map[string]string (for logging)
func getKeys(data map[string]string) []string {
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}
