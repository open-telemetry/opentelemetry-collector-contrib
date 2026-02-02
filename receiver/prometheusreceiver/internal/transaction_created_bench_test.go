// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
)

func BenchmarkAppendWithCreatedLine(b *testing.B) {
	labelSets := generateOMCounterLabelSets(numSeries, 50)
	timestamp := int64(1234567890)
	ctValue := float64(1234567890)

	b.Run("AppendCreatedMetricLine", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			tx := newBenchmarkTransactionForOMCounterBenchmarking(b)
			b.StartTimer()

			for j, ls := range labelSets {
				value := float64(j)
				_, err := tx.Append(0, ls[0], timestamp, value)
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Append(0, ls[1], timestamp, ctValue)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("AppendWithCreatedLineWithAppendSTZeroSample", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			tx := newBenchmarkTransactionForOMCounterBenchmarking(b)
			b.StartTimer()

			for j, ls := range labelSets {
				value := float64(j)
				_, err := tx.Append(0, ls[0], timestamp, value)
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.AppendSTZeroSample(0, ls[1], timestamp, int64(ctValue))
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

// generateOMCounterLabelSets creates label sets for openmetrics style counters with a _total and a _created
func generateOMCounterLabelSets(seriesCount, cardinality int) [][2]labels.Labels {
	result := make([][2]labels.Labels, seriesCount)

	for i := range seriesCount {
		lbls := labels.NewBuilder(labels.EmptyLabels())
		lbls.Set(model.MetricNameLabel, fmt.Sprintf("metric_%d_total", i))

		for j := range cardinality {
			lbls.Set(fmt.Sprintf("label_%d", j), fmt.Sprintf("value_%d_%d", i, j))
		}

		result[i][0] = lbls.Labels()
		lbls.Set(model.MetricNameLabel, fmt.Sprintf("metric_%d_created", i))
		result[i][1] = lbls.Labels()
	}

	return result
}

func newBenchmarkTransactionForOMCounterBenchmarking(b *testing.B) *transaction {
	b.Helper()
	tx := newBenchmarkTransaction(b)
	tx.useMetadata = true
	tx.ctx = scrape.ContextWithMetricMetadataStore(benchCtx, &mockOMCounterMDStore{})
	return tx
}

// mockOMCounterMDStore replicates metadata behavior for counters that follow OpenMetric's _total and_created suffixes
// in which series metadata will be keyed by the normalized metricFamily name (i.e foo_bar_counter_total -> foo_bar_counter)
type mockOMCounterMDStore struct {
	mockMetadataStore
}

func (*mockOMCounterMDStore) GetMetadata(mn string) (scrape.MetricMetadata, bool) {
	if strings.HasSuffix(mn, metricsSuffixBucket) || strings.HasSuffix(mn, metricsSuffixCount) || strings.HasSuffix(mn, metricsSuffixSum) || strings.HasSuffix(mn, metricSuffixInfo) {
		panic("mockOMCounterMDStore should only be used for tests that append openmetrics-style counters")
	}
	// mockOMCounterMDStore only matches normalized counter metric line names without the _total or _created suffix
	if strings.HasSuffix(mn, "_total") || strings.HasSuffix(mn, "_created") {
		return scrape.MetricMetadata{}, false
	}
	nmn := normalizeMetricName(mn)
	return scrape.MetricMetadata{
		MetricFamily: nmn,
		Type:         model.MetricTypeCounter,
	}, true
}
