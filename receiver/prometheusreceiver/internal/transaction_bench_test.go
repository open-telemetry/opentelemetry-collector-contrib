// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// createBenchmarkTransaction creates a transaction for benchmark testing
func createBenchmarkTransaction(tb testing.TB) *transaction {
	target := scrape.NewTarget(
		labels.FromMap(map[string]string{
			model.InstanceLabel: "localhost:8080",
		}),
		&config.ScrapeConfig{},
		map[model.LabelName]model.LabelValue{
			model.AddressLabel: "address:8080",
			model.SchemeLabel:  "http",
		},
		nil,
	)

	scrapeCtx := scrape.ContextWithMetricMetadataStore(
		scrape.ContextWithTarget(context.Background(), target),
		testMetadataStore(testMetadata))

	return newTransaction(
		scrapeCtx,
		&startTimeAdjuster{startTime: startTimestamp},
		new(consumertest.MetricsSink),
		labels.EmptyLabels(),
		receivertest.NewNopSettings(receivertest.NopType),
		nopObsRecv(tb),
		false, // trimSuffixes
		false, // enableNativeHistograms
	)
}

// createTestLabels creates various types of prometheus labels for benchmarking
func createTestLabels(metricName string, additionalLabels ...string) labels.Labels {
	lbls := []string{
		model.MetricNameLabel, metricName,
		model.JobLabel, "benchmark-job",
		model.InstanceLabel, "localhost:8080",
	}
	lbls = append(lbls, additionalLabels...)
	return labels.FromStrings(lbls...)
}

// BenchmarkTransactionAppendAndCommit benchmarks the core processing pipeline (Append + Commit)
func BenchmarkTransactionAppendAndCommit(b *testing.B) {
	b.Run("Floats", func(b *testing.B) {
		benchmarkFloat(b)
	})
	b.Run("Histogram", func(b *testing.B) {
		benchmarkHistogram(b)
	})
}

func benchmarkFloat(b *testing.B) {
	tr := createBenchmarkTransaction(b)
	metricName := "metric"
	labels := []string{"method", "GET", "status", "200"}
	lbls := createTestLabels(metricName, labels...)
	timestamp := time.Now().Unix() * 1000

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := tr.Append(0, lbls, timestamp, float64(i))
		require.NoError(b, err)

		targetInfoLabels := createTestLabels("target_info",
			"version", "1.2.3",
			"service_name", "benchmark-service",
			"environment", "test")
		_, err = tr.Append(0, targetInfoLabels, timestamp, 1.0)
		require.NoError(b, err)

		scopeInfoLabels := createTestLabels("otel_scope_info",
			"otel_scope_name", "benchmark-scope",
			"otel_scope_version", "1.0.0",
			"otel_scope_schema_url", "https://example.com/schema",
			"custom_attr", "benchmark_value")
		_, err = tr.Append(0, scopeInfoLabels, timestamp, 1.0)
		require.NoError(b, err)

		newScopeLabels := createTestLabels(metricName+"_with_scope_labels",
			append(labels,
				"otel_scope_name", "benchmark-scope",
				"otel_scope_version", "1.0.0",
				"otel_scope_schema_url", "https://example.com/schema",
				"otel_scope_custom_attr", "benchmark_value")...)
		_, err = tr.Append(0, newScopeLabels, timestamp, float64(i))
		require.NoError(b, err)
	}

	err := tr.Commit()
	require.NoError(b, err)
}

func benchmarkHistogram(b *testing.B) {
	tr := createBenchmarkTransaction(b)

	// Create histogram buckets
	buckets := []string{
		"le", "0.1",
		"le", "0.5",
		"le", "1.0",
		"le", "5.0",
		"le", "+Inf",
	}

	timestamp := time.Now().Unix() * 1000

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		targetInfoLabels := createTestLabels("target_info",
			"version", "2.1.0",
			"service_name", "web-api",
			"environment", "production",
			"region", "us-west-2")
		_, err := tr.Append(0, targetInfoLabels, timestamp, 1.0)
		require.NoError(b, err)

		scopeInfoLabels := createTestLabels("otel_scope_info",
			"otel_scope_name", "http-server",
			"otel_scope_version", "2.1.0",
			"otel_scope_schema_url", "https://opentelemetry.io/schemas/1.17.0",
			"service_name", "web-api")
		_, err = tr.Append(0, scopeInfoLabels, timestamp, 1.0)
		require.NoError(b, err)

		for j := 0; j < len(buckets); j += 2 {
			lbls := createTestLabels("http_duration_bucket", "method", "GET", buckets[j], buckets[j+1])
			_, err = tr.Append(0, lbls, timestamp, float64(i*10+j/2))
			require.NoError(b, err)
		}

		sumLabels := createTestLabels("http_duration_sum", "method", "GET")
		_, err = tr.Append(0, sumLabels, timestamp, float64(i*100))
		require.NoError(b, err)

		countLabels := createTestLabels("http_duration_count", "method", "GET")
		_, err = tr.Append(0, countLabels, timestamp, float64(i*20))
		require.NoError(b, err)

		newScopeBucketLabels := createTestLabels("request_duration_bucket_with_scope",
			"method", "POST",
			"le", "1.0",
			"otel_scope_name", "http-server",
			"otel_scope_version", "2.1.0",
			"otel_scope_schema_url", "https://opentelemetry.io/schemas/1.17.0",
			"otel_scope_service_name", "web-api")
		_, err = tr.Append(0, newScopeBucketLabels, timestamp, float64(i*5))
		require.NoError(b, err)
	}

	err := tr.Commit()
	require.NoError(b, err)
}
