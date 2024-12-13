// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

func TestCAdvisorMetric_Merge(t *testing.T) {
	src := &stores.CIMetricImpl{
		Fields: map[string]any{"value1": 1, "value2": 2},
		Tags:   map[string]string{ci.Timestamp: "1586331559882"},
		Logger: zap.NewNop(),
	}
	dest := &stores.CIMetricImpl{
		Fields: map[string]any{"value1": 3, "value3": 3},
		Tags:   map[string]string{ci.Timestamp: "1586331559973"},
		Logger: zap.NewNop(),
	}
	src.Merge(dest)
	assert.Len(t, src.Fields, 3)
	assert.Equal(t, 1, src.Fields["value1"].(int))
}

func TestGetMetricKey(t *testing.T) {
	c := &stores.CIMetricImpl{
		Tags: map[string]string{
			ci.MetricType: ci.TypeInstance,
		},
	}
	assert.Equal(t, "metricType:Instance", getMetricKey(c))

	c = &stores.CIMetricImpl{
		Tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}
	assert.Equal(t, "metricType:Node", getMetricKey(c))

	c = &stores.CIMetricImpl{
		Tags: map[string]string{
			ci.MetricType: ci.TypePod,
			ci.PodIDKey:   "podID",
		},
	}
	assert.Equal(t, "metricType:Pod,podId:podID", getMetricKey(c))

	c = &stores.CIMetricImpl{
		Tags: map[string]string{
			ci.MetricType:       ci.TypeContainer,
			ci.PodIDKey:         "podID",
			ci.ContainerNamekey: "containerName",
		},
	}
	// TODO(kausyas): Make sure this isnt exported anywhere
	assert.Equal(t, "metricType:Container,podId:podID,containerName:containerName", getMetricKey(c))

	c = &stores.CIMetricImpl{
		Tags: map[string]string{
			ci.MetricType: ci.TypeInstanceDiskIO,
			ci.DiskDev:    "/abc",
		},
	}
	assert.Equal(t, "metricType:InstanceDiskIO,device:/abc", getMetricKey(c))

	c = &stores.CIMetricImpl{
		Tags: map[string]string{
			ci.MetricType: ci.TypeNodeDiskIO,
			ci.DiskDev:    "/abc",
		},
	}
	assert.Equal(t, "metricType:NodeDiskIO,device:/abc", getMetricKey(c))

	c = &stores.CIMetricImpl{}
	assert.Equal(t, "", getMetricKey(c))
}

func TestMergeMetrics(t *testing.T) {
	cpuMetrics := &stores.CIMetricImpl{
		Fields: map[string]any{
			"node_cpu_usage_total": float64(10),
			"node_cpu_usage_user":  float64(10),
		},
		Tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}

	memMetrics := &stores.CIMetricImpl{
		Fields: map[string]any{
			"node_memory_cache": uint(25645056),
		},
		Tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}

	metrics := []*stores.CIMetricImpl{
		cpuMetrics,
		memMetrics,
	}

	expected := &stores.CIMetricImpl{
		Fields: map[string]any{
			"node_cpu_usage_total": float64(10),
			"node_cpu_usage_user":  float64(10),
			"node_memory_cache":    uint(25645056),
		},
		Tags: map[string]string{
			ci.MetricType: ci.TypeNode,
		},
	}
	mergedMetrics := MergeMetrics(metrics)
	require.Len(t, mergedMetrics, 1)
	assert.Equal(t, expected.GetTags(), mergedMetrics[0].GetTags())
	assert.Equal(t, expected.GetFields(), mergedMetrics[0].GetFields())
}
