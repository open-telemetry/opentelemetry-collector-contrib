// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

func TestCAdvisorMetric_Merge(t *testing.T) {
	src := &CAdvisorMetric{
		fields: map[string]any{"value1": 1, "value2": 2},
		tags:   map[string]string{ci.Timestamp: "1586331559882"},
		logger: zap.NewNop(),
	}
	dest := &CAdvisorMetric{
		fields: map[string]any{"value1": 3, "value3": 3},
		tags:   map[string]string{ci.Timestamp: "1586331559973"},
		logger: zap.NewNop(),
	}
	src.Merge(dest)
	assert.Equal(t, 3, len(src.fields))
	assert.Equal(t, 1, src.fields["value1"].(int))
}

func TestGetMetricKey(t *testing.T) {
	c := &CAdvisorMetric{
		tags: map[string]string{
			ci.MetricType: ci.TypeInstance,
		},
	}
	assert.Equal(t, "metricType:Instance", getMetricKey(c))

	c = &CAdvisorMetric{
		tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}
	assert.Equal(t, "metricType:Node", getMetricKey(c))

	c = &CAdvisorMetric{
		tags: map[string]string{
			ci.MetricType: ci.TypePod,
			ci.PodIDKey:   "podID",
		},
	}
	assert.Equal(t, "metricType:Pod,podId:podID", getMetricKey(c))

	c = &CAdvisorMetric{
		tags: map[string]string{
			ci.MetricType:       ci.TypeContainer,
			ci.PodIDKey:         "podID",
			ci.ContainerNamekey: "containerName",
		},
	}
	assert.Equal(t, "metricType:Container,podId:podID,containerName:containerName", getMetricKey(c))

	c = &CAdvisorMetric{
		tags: map[string]string{
			ci.MetricType: ci.TypeInstanceDiskIO,
			ci.DiskDev:    "/abc",
		},
	}
	assert.Equal(t, "metricType:InstanceDiskIO,device:/abc", getMetricKey(c))

	c = &CAdvisorMetric{
		tags: map[string]string{
			ci.MetricType: ci.TypeNodeDiskIO,
			ci.DiskDev:    "/abc",
		},
	}
	assert.Equal(t, "metricType:NodeDiskIO,device:/abc", getMetricKey(c))

	c = &CAdvisorMetric{}
	assert.Equal(t, "", getMetricKey(c))
}

func TestMergeMetrics(t *testing.T) {
	cpuMetrics := &CAdvisorMetric{
		fields: map[string]any{
			"node_cpu_usage_total": float64(10),
			"node_cpu_usage_user":  float64(10),
		},
		tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}

	memMetrics := &CAdvisorMetric{
		fields: map[string]any{
			"node_memory_cache": uint(25645056),
		},
		tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}

	metrics := []*CAdvisorMetric{
		cpuMetrics,
		memMetrics,
	}

	expected := &CAdvisorMetric{
		fields: map[string]any{
			"node_cpu_usage_total": float64(10),
			"node_cpu_usage_user":  float64(10),
			"node_memory_cache":    uint(25645056),
		},
		tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}
	mergedMetrics := MergeMetrics(metrics)
	require.Len(t, mergedMetrics, 1)
	assert.Equal(t, expected.GetTags(), mergedMetrics[0].GetTags())
	assert.Equal(t, expected.GetFields(), mergedMetrics[0].GetFields())

}
