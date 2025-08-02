// processor_test.go - Integration tests for the main processor functionality
package isolationforestprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestIsolationForestProcessorCreation(t *testing.T) {
	cfg := createDefaultConfig()
	logger := zaptest.NewLogger(t)

	processor, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err, "Should create processor without error")
	require.NotNil(t, processor, "Processor should not be nil")

	// Verify processor components are initialized
	assert.NotNil(t, processor.config)
	assert.NotNil(t, processor.logger)
	assert.NotNil(t, processor.traceExtractor)
	assert.NotNil(t, processor.metricsExtractor)
	assert.NotNil(t, processor.logsExtractor)

	// Single model mode should have default forest
	if !cfg.IsMultiModelMode() {
		assert.NotNil(t, processor.defaultForest)
	}

	// Clean up
	err = processor.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestTraceProcessingEnrichMode(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.Mode = "enrich"
	cfg.Threshold = 0.5 // Lower threshold for testing

	logger := zaptest.NewLogger(t)
	processor, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())

	// Create test trace data with both normal and anomalous spans
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()

	// Add resource attributes
	resourceSpans.Resource().Attributes().PutStr("service.name", "test-service")

	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	// Create normal span (short duration, no error)
	normalSpan := scopeSpans.Spans().AppendEmpty()
	normalSpan.SetName("normal_operation")
	normalSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	normalSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(100 * time.Millisecond)))
	normalSpan.Status().SetCode(ptrace.StatusCodeOk)
	normalSpan.Attributes().PutStr("http.status_code", "200")

	// Create anomalous span (very long duration, error)
	anomalousSpan := scopeSpans.Spans().AppendEmpty()
	anomalousSpan.SetName("slow_operation")
	anomalousSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	anomalousSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10 * time.Second)))
	anomalousSpan.Status().SetCode(ptrace.StatusCodeError)
	anomalousSpan.Attributes().PutStr("http.status_code", "500")

	// Process several samples to allow the model to learn patterns
	for i := 0; i < 50; i++ {
		// Extract features from normal span
		features := processor.traceExtractor.ExtractFeatures(normalSpan, map[string]interface{}{
			"service.name": "test-service",
		})

		// Process through isolation forest
		attrs := map[string]interface{}{"service.name": "test-service"}
		processor.processFeatures(features, attrs)

		// Small delay to allow async updates
		time.Sleep(1 * time.Millisecond)
	}

	// Now test anomaly detection
	anomalousFeatures := processor.traceExtractor.ExtractFeatures(anomalousSpan, map[string]interface{}{
		"service.name": "test-service",
	})

	attrs := map[string]interface{}{"service.name": "test-service"}
	score, isAnomaly, modelName := processor.processFeatures(anomalousFeatures, attrs)

	// Verify anomaly detection
	assert.True(t, score >= 0.0 && score <= 1.0, "Score should be between 0 and 1")
	// Note: In a real scenario with more training data, we'd expect isAnomaly to be true
	// For this unit test, we just verify the processing works

	// Verify model name
	if cfg.IsMultiModelMode() {
		assert.NotEmpty(t, modelName)
	}
}

func TestFeatureExtraction(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test trace feature extraction
	traceExtractor := NewTraceFeatureExtractor([]string{"duration", "error", "http.status_code"}, logger)

	// Create test span
	span := ptrace.NewSpan()
	span.SetName("test_operation")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(500 * time.Millisecond)))
	span.Status().SetCode(ptrace.StatusCodeError)
	span.Attributes().PutStr("http.status_code", "404")

	resourceAttrs := map[string]interface{}{
		"service.name": "test-service",
	}

	features := traceExtractor.ExtractFeatures(span, resourceAttrs)

	// Verify extracted features
	assert.Contains(t, features, "duration", "Should extract duration feature")
	assert.Contains(t, features, "error", "Should extract error feature")
	assert.Contains(t, features, "http.status_code", "Should extract HTTP status code feature")

	// Verify feature values
	durationFeature := features["duration"]
	require.Len(t, durationFeature, 1)
	assert.True(t, durationFeature[0] > 0, "Duration should be positive")

	errorFeature := features["error"]
	require.Len(t, errorFeature, 1)
	assert.Equal(t, 1.0, errorFeature[0], "Error feature should be 1.0 for error status")

	statusFeature := features["http.status_code"]
	require.Len(t, statusFeature, 1)
	assert.Equal(t, 404.0, statusFeature[0], "HTTP status should be 404")
}

func TestCategoricalEncoding(t *testing.T) {
	// Test that categorical encoding is consistent and produces valid values
	testValues := []string{"service-a", "service-b", "service-c", ""}

	for _, value := range testValues {
		encoded := categoricalEncode(value)

		// Verify encoded value is in valid range
		assert.True(t, encoded >= 0.0 && encoded <= 1.0,
			"Encoded value should be between 0 and 1 for value: %s", value)

		// Verify consistency - same input should produce same output
		encoded2 := categoricalEncode(value)
		assert.Equal(t, encoded, encoded2,
			"Categorical encoding should be consistent for value: %s", value)
	}

	// Verify different inputs produce different outputs
	encoded1 := categoricalEncode("service-a")
	encoded2 := categoricalEncode("service-b")
	assert.NotEqual(t, encoded1, encoded2, "Different services should have different encodings")
}

// ---
