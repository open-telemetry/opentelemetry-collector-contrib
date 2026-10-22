// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver

import (
	"context"
	"os"
	"testing"

	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

type benchmarkRestClient struct {
	statsSummary []byte
	pods         []byte
}

func (c *benchmarkRestClient) StatsSummary() ([]byte, error) {
	return c.statsSummary, nil
}

func (c *benchmarkRestClient) Pods() ([]byte, error) {
	return c.pods, nil
}

func BenchmarkKubeletStatsReceiverScrapeMetrics(b *testing.B) {
	statsSummary, err := os.ReadFile("testdata/stats-summary.json")
	if err != nil {
		b.Fatal(err)
	}

	pods, err := os.ReadFile("testdata/pods.json")
	if err != nil {
		b.Fatal(err)
	}

	restClient := &benchmarkRestClient{
		statsSummary: statsSummary,
		pods:         pods,
	}

	options := &scraperOptions{
		metricGroupsToCollect: map[kubelet.MetricGroup]bool{
			kubelet.ContainerMetricGroup: true,
			kubelet.PodMetricGroup:       true,
			kubelet.NodeMetricGroup:      true,
		},
	}

	scraper, err := newKubeletScraper(
		restClient,
		receivertest.NewNopSettings(metadata.Type),
		options,
		metadata.NewDefaultMetricsBuilderConfig(),
		"worker-42",
	)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := scraper.ScrapeMetrics(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
