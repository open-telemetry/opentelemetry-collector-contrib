// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver

import "testing"

func TestGetMetricTimeGrain(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		metric       string
		overrides    MetricTimeGrainOverrides
		defaultGrain string
		want         string
	}{
		{
			name:         "uses Azure default when no override is configured",
			namespace:    "Microsoft.ElasticSan/elasticSans",
			metric:       "ElasticSanProvisionedBase",
			defaultGrain: "PT1M",
			want:         "PT1M",
		},
		{
			name:      "uses a configured override",
			namespace: "Microsoft.ElasticSan/elasticSans",
			metric:    "ElasticSanProvisionedBase",
			overrides: MetricTimeGrainOverrides{
				"Microsoft.ElasticSan/elasticSans": map[string]string{"ElasticSanProvisionedBase": "PT30M"},
			},
			defaultGrain: "PT1M",
			want:         "PT30M",
		},
		{
			name:      "looks up namespace and metric case insensitively",
			namespace: "Microsoft.ElasticSan/elasticSans",
			metric:    "ElasticSanProvisionedBase",
			overrides: MetricTimeGrainOverrides{
				"microsoft.elasticsan/elasticSans": map[string]string{"elasticsanprovisionedbase": "pt30m"},
			},
			defaultGrain: "PT1M",
			want:         "PT30M",
		},
		{
			name:      "keeps default for another metric",
			namespace: "Microsoft.ElasticSan/elasticSans",
			metric:    "OtherMetric",
			overrides: MetricTimeGrainOverrides{
				"Microsoft.ElasticSan/elasticSans": map[string]string{"ElasticSanProvisionedBase": "PT30M"},
			},
			defaultGrain: "PT1M",
			want:         "PT1M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMetricTimeGrain(tt.namespace, tt.metric, tt.overrides, tt.defaultGrain)
			if got != tt.want {
				t.Fatalf("getMetricTimeGrain() = %q, want %q", got, tt.want)
			}
		})
	}
}
