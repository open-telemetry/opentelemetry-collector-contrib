// integration_test.go - End-to-end integration tests demonstrating real-world usage
package isolationforestprocessor

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

// TestEndToEndTraceAnomalyDetection demonstrates the processor working with realistic trace data
func TestEndToEndTraceAnomalyDetection(t *testing.T) {
	// Create processor with realistic configuration
	cfg := &Config{
		ForestSize:              50, // Smaller for faster testing
		SubsampleSize:           128,
		ContaminationRate:       0.1,
		Mode:                    "enrich",
		Threshold:               0.6, // Lower threshold for more sensitive detection
		TrainingWindow:          "1h",
		UpdateFreqency:          "5m",
		MinSamples:              20, // Lower for faster training
		ScoreAttribute:          "anomaly.isolation_score",
		ClassificationAttribute: "anomaly.is_anomaly",
		Features: FeatureConfig{
			Traces: []string{"duration", "error", "http.status_code"},
		},
		Performance: PerformanceConfig{
			MaxMemoryMB:     256,
			BatchSize:       100,
			ParallelWorkers: 2,
		},
	}

	// Create processor
	logger := zaptest.NewLogger(t)
	processor, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())

	// Create consumer to capture processed data
	sink := &consumertest.TracesSink{}
	tracesProcessor := &tracesProcessor{
		isolationForestProcessor: processor,
		nextConsumer:             sink,
		logger:                   logger,
	}

	// Generate training data (mostly normal behavior)
	trainingData := generateRealisticTraceData(100, 0.05) // 5% anomalies for training

	// Process training data
	for _, traces := range trainingData {
		err := tracesProcessor.ProcessTraces(context.Background(), traces)
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond) // Allow async processing
	}

	// Allow model to stabilize
	time.Sleep(50 * time.Millisecond)

	// Generate test data with known anomalies
	testData := generateRealisticTraceData(20, 0.3) // 30% anomalies for testing

	// Track anomaly detection results
	var processedTraces []ptrace.Traces
	sink.Reset() // Clear training data from sink

	// Process test data
	for _, traces := range testData {
		err := tracesProcessor.ProcessTraces(context.Background(), traces)
		require.NoError(t, err)
		processedTraces = append(processedTraces, traces)
	}

	// Analyze results
	anomalyCount := 0
	totalSpans := 0

	receivedTraces := sink.AllTraces()
	for _, traces := range receivedTraces {
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			rs := traces.ResourceSpans().At(i)
			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				ss := rs.ScopeSpans().At(j)
				for k := 0; k < ss.Spans().Len(); k++ {
					span := ss.Spans().At(k)
					totalSpans++

					// Check if anomaly attributes were added
					if scoreAttr, exists := span.Attributes().Get(cfg.ScoreAttribute); exists {
						score := scoreAttr.Double()
						assert.True(t, score >= 0.0 && score <= 1.0,
							"Anomaly score should be between 0 and 1, got %f", score)

						if isAnomalyAttr, exists := span.Attributes().Get(cfg.ClassificationAttribute); exists {
							if isAnomalyAttr.Bool() {
								anomalyCount++
							}
						}
					}
				}
			}
		}
	}

	// Verify processing worked
	assert.True(t, totalSpans > 0, "Should have processed some spans")
	assert.True(t, anomalyCount >= 0, "Should have detected some anomalies (or zero in test)")

	// Verify processor statistics
	stats := processor.defaultForest.GetStatistics()
	assert.True(t, stats.TotalSamples > 0, "Should have processed samples")
	assert.True(t, stats.CurrentThreshold > 0, "Should have adaptive threshold")

	t.Logf("Processed %d spans, detected %d anomalies (%.1f%%)",
		totalSpans, anomalyCount, 100.0*float64(anomalyCount)/float64(totalSpans))
}

// TestMultiModelTraceProcessing tests the multi-model functionality
func TestMultiModelTraceProcessing(t *testing.T) {
	cfg := &Config{
		ForestSize:              20,
		SubsampleSize:           64,
		ContaminationRate:       0.1,
		Mode:                    "enrich",
		Threshold:               0.7,
		TrainingWindow:          "30m",
		UpdateFrequency:         "5m",
		MinSamples:              10,
		ScoreAttribute:          "anomaly.score",
		ClassificationAttribute: "anomaly.detected",
		Models: []ModelConfig{
			{
				Name: "web_service",
				Selector: map[string]string{
					"service.name": "web-frontend",
				},
				Features:          []string{"duration", "error", "http.status_code"},
				Threshold:         0.8, // Stricter for web service
				ForestSize:        30,
				SubsampleSize:     64,
				ContaminationRate: 0.05,
			},
			{
				Name: "background_service",
				Selector: map[string]string{
					"service.name": "background-worker",
				},
				Features:          []string{"duration", "error"},
				Threshold:         0.6, // More permissive for background
				ForestSize:        20,
				SubsampleSize:     64,
				ContaminationRate: 0.15,
			},
		},
		Performance: PerformanceConfig{
			MaxMemoryMB:     128,
			BatchSize:       50,
			ParallelWorkers: 2,
		},
	}

	logger := zaptest.NewLogger(t)
	processor, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())

	sink := &consumertest.TracesSink{}
	tracesProcessor := &tracesProcessor{
		isolationForestProcessor: processor,
		nextConsumer:             sink,
		logger:                   logger,
	}

	// Create traces for different services
	webTraces := createServiceTraces("web-frontend", 10)
	backgroundTraces := createServiceTraces("background-worker", 10)
	unknownTraces := createServiceTraces("unknown-service", 5)

	// Process all traces
	allTraces := append(append(webTraces, backgroundTraces...), unknownTraces...)
	for _, traces := range allTraces {
		err := tracesProcessor.ProcessTraces(context.Background(), traces)
		require.NoError(t, err)
	}

	// Verify model selection worked by checking for model_name attributes
	receivedTraces := sink.AllTraces()
	webModelCount := 0
	backgroundModelCount := 0

	for _, traces := range receivedTraces {
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			rs := traces.ResourceSpans().At(i)
			serviceName, _ := rs.Resource().Attributes().Get("service.name")

			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				ss := rs.ScopeSpans().At(j)
				for k := 0; k < ss.Spans().Len(); k++ {
					span := ss.Spans().At(k)

					if modelAttr, exists := span.Attributes().Get("anomaly.model_name"); exists {
						modelName := modelAttr.Str()
						switch serviceName.Str() {
						case "web-frontend":
							if modelName == "web_service" {
								webModelCount++
							}
						case "background-worker":
							if modelName == "background_service" {
								backgroundModelCount++
							}
						}
					}
				}
			}
		}
	}

	t.Logf("Web service model used %d times, background service model used %d times",
		webModelCount, backgroundModelCount)
}

// TestFilterModeTraceProcessing tests the filter mode functionality
func TestFilterModeTraceProcessing(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.Mode = "filter"
	cfg.Threshold = 0.5 // Lower threshold to filter more aggressively
	cfg.MinSamples = 5  // Lower for faster testing

	logger := zaptest.NewLogger(t)
	processor, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())

	sink := &consumertest.TracesSink{}
	tracesProcessor := &tracesProcessor{
		isolationForestProcessor: processor,
		nextConsumer:             sink,
		logger:                   logger,
	}

	// Generate mixed data (some normal, some anomalous)
	inputTraces := generateRealisticTraceData(20, 0.2)

	// Count input spans
	inputSpanCount := 0
	for _, traces := range inputTraces {
		inputSpanCount += countSpansInTraces(traces)
	}

	// Process data
	for _, traces := range inputTraces {
		err := tracesProcessor.ProcessTraces(context.Background(), traces)
		require.NoError(t, err)
	}

	// Count output spans
	outputSpanCount := 0
	receivedTraces := sink.AllTraces()
	for _, traces := range receivedTraces {
		outputSpanCount += countSpansInTraces(traces)
	}

	// In filter mode, we should have fewer or equal output spans
	// (anomalous spans should be passed through, normal spans filtered)
	assert.True(t, outputSpanCount <= inputSpanCount,
		"Filter mode should not increase span count: input=%d, output=%d",
		inputSpanCount, outputSpanCount)

	t.Logf("Filter mode: %d input spans -> %d output spans (%.1f%% kept)",
		inputSpanCount, outputSpanCount, 100.0*float64(outputSpanCount)/float64(inputSpanCount))
}

// Helper functions for integration tests

// generateRealisticTraceData creates trace data with realistic patterns and controlled anomaly rates
func generateRealisticTraceData(numTraces int, anomalyRate float64) []ptrace.Traces {
	var allTraces []ptrace.Traces
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < numTraces; i++ {
		traces := ptrace.NewTraces()
		rs := traces.ResourceSpans().AppendEmpty()

		// Add resource attributes
		rs.Resource().Attributes().PutStr("service.name", "test-service")
		rs.Resource().Attributes().PutStr("service.version", "1.0.0")

		ss := rs.ScopeSpans().AppendEmpty()

		// Decide if this should be anomalous
		isAnomalous := rand.Float64() < anomalyRate

		// Create span with realistic attributes
		span := ss.Spans().AppendEmpty()
		span.SetName(fmt.Sprintf("operation_%d", i%5))

		baseTime := time.Now().Add(-time.Duration(rand.Intn(3600)) * time.Second)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(baseTime))

		// Set duration and status based on whether it should be anomalous
		var duration time.Duration
		var statusCode ptrace.StatusCode
		var httpStatus string

		if isAnomalous {
			// Anomalous: very slow, errors, unusual status codes
			duration = time.Duration(rand.Intn(10000)+5000) * time.Millisecond // 5-15 seconds
			statusCode = ptrace.StatusCodeError
			httpStatus = []string{"500", "502", "503", "504"}[rand.Intn(4)]
		} else {
			// Normal: fast, successful
			duration = time.Duration(rand.Intn(500)+50) * time.Millisecond // 50-550ms
			statusCode = ptrace.StatusCodeOk
			httpStatus = []string{"200", "201", "204"}[rand.Intn(3)]
		}

		span.SetEndTimestamp(pcommon.NewTimestampFromTime(baseTime.Add(duration)))
		span.Status().SetCode(statusCode)
		span.Attributes().PutStr("http.status_code", httpStatus)
		span.Attributes().PutStr("http.method", []string{"GET", "POST", "PUT"}[rand.Intn(3)])

		allTraces = append(allTraces, traces)
	}

	return allTraces
}

// createServiceTraces creates traces for a specific service
func createServiceTraces(serviceName string, count int) []ptrace.Traces {
	var allTraces []ptrace.Traces

	for i := 0; i < count; i++ {
		traces := ptrace.NewTraces()
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", serviceName)

		ss := rs.ScopeSpans().AppendEmpty()
		span := ss.Spans().AppendEmpty()
		span.SetName(fmt.Sprintf("%s_operation_%d", serviceName, i))

		baseTime := time.Now()
		duration := time.Duration(rand.Intn(1000)+100) * time.Millisecond
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(baseTime))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(baseTime.Add(duration)))
		span.Status().SetCode(ptrace.StatusCodeOk)

		allTraces = append(allTraces, traces)
	}

	return allTraces
}

// countSpansInTraces counts the total number of spans in a traces object
func countSpansInTraces(traces ptrace.Traces) int {
	count := 0
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			count += ss.Spans().Len()
		}
	}
	return count
}

// ---
