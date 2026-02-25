// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestReplacePatternValidTaskId(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskId}"

	attrMap := map[string]any{
		"aws.ecs.cluster.name": "test-cluster-name",
		"aws.ecs.task.id":      "test-task-id",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "test-task-id", s)
	assert.True(t, success)
}

func TestReplacePatternValidServiceName(t *testing.T) {
	logger := zap.NewNop()

	input := "{ServiceName}"

	attrMap := map[string]any{
		"service.name": "some-test-service",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "some-test-service", s)
	assert.True(t, success)
}

func TestReplacePatternValidClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := map[string]any{
		"aws.ecs.cluster.name": "test-cluster-name",
		"aws.ecs.task.id":      "test-task-id",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
	assert.True(t, success)
}

func TestReplacePatternMissingAttribute(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := map[string]any{
		"aws.ecs.task.id": "test-task-id",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternValidPodName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	attrMap := map[string]any{
		"aws.eks.cluster.name": "test-cluster-name",
		"PodName":              "test-pod-001",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/test-pod-001/performance", s)
	assert.True(t, success)
}

func TestReplacePatternValidPod(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	attrMap := map[string]any{
		"aws.eks.cluster.name": "test-cluster-name",
		"PodName":              "test-pod-001",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/test-pod-001/performance", s)
	assert.True(t, success)
}

func TestReplacePatternMissingPodName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	attrMap := map[string]any{
		"aws.eks.cluster.name": "test-cluster-name",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternAttrPlaceholderClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := map[string]any{
		"ClusterName": "test-cluster-name",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
	assert.True(t, success)
}

func TestReplacePatternWrongKey(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{WrongKey}/performance"

	attrMap := map[string]any{
		"ClusterName": "test-task-id",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/{WrongKey}/performance", s)
	assert.True(t, success)
}

func TestReplacePatternNilAttrValue(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := map[string]any{
		"ClusterName": "",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternValidTaskDefinitionFamily(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskDefinitionFamily}"

	attrMap := map[string]any{
		"aws.ecs.cluster.name": "test-cluster-name",
		"aws.ecs.task.family":  "test-task-definition-family",
	}

	s, success := replacePatterns(input, anyMapToStringMap(attrMap), logger)

	assert.Equal(t, "test-task-definition-family", s)
	assert.True(t, success)
}

func TestIsPatternValid(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{
			name:     "no curly brackets",
			pattern:  "example-string",
			expected: true,
		},
		{
			name:     "valid single pattern",
			pattern:  "prefix-{ClusterName}-suffix",
			expected: true,
		},
		{
			name:     "valid multiple patterns",
			pattern:  "{ServiceName}-{TaskId}-{FaasName}",
			expected: true,
		},
		{
			name:     "invalid pattern key",
			pattern:  "prefix-{RandomName}-suffix",
			expected: false,
		},
		{
			name:     "mixed valid and invalid",
			pattern:  "{ClusterName}-{RandomName}",
			expected: false,
		},
		{
			name:     "empty curly brackets",
			pattern:  "prefix-{}-suffix",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := isPatternValid(tc.pattern)
			assert.Equal(t, tc.expected, result, "Pattern: %s", tc.pattern)
		})
	}
}
