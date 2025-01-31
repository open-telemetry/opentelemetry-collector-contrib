// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
)

func TestReplacePatternValidTaskId(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskId}"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("aws.ecs.cluster.name", "test-cluster-name")
	//attrMap.PutStr("aws.ecs.task.id", "test-task-id")

	attrMap := map[string]any{
		"aws.ecs.cluster.name": "test-cluster-name",
		"aws.ecs.task.id":      "test-task-id",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "test-task-id", s)
	assert.True(t, success)
}

func TestReplacePatternValidServiceName(t *testing.T) {
	logger := zap.NewNop()

	input := "{ServiceName}"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("service.name", "some-test-service")

	attrMap := map[string]any{
		"service.name": "some-test-service",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "some-test-service", s)
	assert.True(t, success)
}

func TestReplacePatternValidClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("aws.ecs.cluster.name", "test-cluster-name")
	//attrMap.PutStr("aws.ecs.task.id", "test-task-id")

	attrMap := map[string]any{
		"aws.ecs.cluster.name": "test-cluster-name",
		"aws.ecs.task.id":      "test-task-id",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
	assert.True(t, success)
}

func TestReplacePatternMissingAttribute(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("aws.ecs.task.id", "test-task-id")

	attrMap := map[string]any{
		"aws.ecs.task.id": "test-task-id",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternValidPodName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("aws.eks.cluster.name", "test-cluster-name")
	//attrMap.PutStr("PodName", "test-pod-001")

	attrMap := map[string]any{
		"aws.eks.cluster.name": "test-cluster-name",
		"PodName":              "test-pod-001",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/test-pod-001/performance", s)
	assert.True(t, success)
}

func TestReplacePatternValidPod(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("aws.eks.cluster.name", "test-cluster-name")
	//attrMap.PutStr("pod", "test-pod-001")

	attrMap := map[string]any{
		"aws.eks.cluster.name": "test-cluster-name",
		"PodName":              "test-pod-001",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/test-pod-001/performance", s)
	assert.True(t, success)
}

func TestReplacePatternMissingPodName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("aws.eks.cluster.name", "test-cluster-name")

	attrMap := map[string]any{
		"aws.eks.cluster.name": "test-cluster-name",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternAttrPlaceholderClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("ClusterName", "test-cluster-name")

	attrMap := map[string]any{
		"ClusterName": "test-cluster-name",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
	assert.True(t, success)
}

func TestReplacePatternWrongKey(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{WrongKey}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("ClusterName", "test-task-id")

	attrMap := map[string]any{
		"ClusterName": "test-task-id",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/{WrongKey}/performance", s)
	assert.True(t, success)
}

func TestReplacePatternNilAttrValue(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	//attrMap := pcommon.NewMap()
	//attrMap.PutEmpty("ClusterName")

	attrMap := map[string]any{
		"ClusterName": "",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternValidTaskDefinitionFamily(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskDefinitionFamily}"

	//attrMap := pcommon.NewMap()
	//attrMap.PutStr("aws.ecs.cluster.name", "test-cluster-name")
	//attrMap.PutStr("aws.ecs.task.family", "test-task-definition-family")

	attrMap := map[string]any{
		"aws.ecs.cluster.name": "test-cluster-name",
		"aws.ecs.task.family":  "test-task-definition-family",
	}

	s, success := replacePatterns(input, anyMaptoStringMap(attrMap), logger)

	assert.Equal(t, "test-task-definition-family", s)
	assert.True(t, success)
}

func TestGetLogInfo(t *testing.T) {
	rm1 := pmetric.NewResourceMetrics()
	rm1.Resource().Attributes().PutStr(conventions.AttributeServiceName, "myServiceName")
	rm1.Resource().Attributes().PutStr(occonventions.AttributeExporterVersion, "SomeVersion")
	rm1.Resource().Attributes().PutStr("aws.ecs.cluster.name", "test-cluster-name")
	rm1.Resource().Attributes().PutStr("aws.ecs.task.id", "test-task-id")
	rm1.Resource().Attributes().PutStr("k8s.node.name", "ip-192-168-58-245.ec2.internal")
	rm1.Resource().Attributes().PutStr("aws.ecs.container.instance.id", "203e0410260d466bab7873bb4f317b4e")
	rm1.Resource().Attributes().PutStr("aws.ecs.task.family", "test-task-definition-family")
	rm2 := pmetric.NewResourceMetrics()
	rm2.Resource().Attributes().PutStr(conventions.AttributeServiceName, "test-emf")
	rm2.Resource().Attributes().PutStr(occonventions.AttributeExporterVersion, "SomeVersion")
	rm2.Resource().Attributes().PutStr("ClusterName", "test-cluster-name")
	rm2.Resource().Attributes().PutStr("TaskId", "test-task-id")
	rm2.Resource().Attributes().PutStr("NodeName", "ip-192-168-58-245.ec2.internal")
	rm2.Resource().Attributes().PutStr("ContainerInstanceId", "203e0410260d466bab7873bb4f317b4e")
	rm2.Resource().Attributes().PutStr("TaskDefinitionFamily", "test-task-definition-family")
	rms := []pmetric.ResourceMetrics{rm1, rm2}

	testCases := []struct {
		testName        string
		namespace       string
		configLogGroup  string
		configLogStream string
		logGroup        string
		logStream       string
	}{
		{
			"non-empty namespace, no config",
			"namespace",
			"",
			"",
			"/metrics/namespace",
			"",
		},
		{
			"empty namespace, no config",
			"",
			"",
			"",
			"",
			"",
		},
		{
			"non-empty namespace, config w/o pattern",
			"namespace",
			"test-logGroupName",
			"test-logStreamName",
			"test-logGroupName",
			"test-logStreamName",
		},
		{
			"empty namespace, config w/o pattern",
			"",
			"test-logGroupName",
			"test-logStreamName",
			"test-logGroupName",
			"test-logStreamName",
		},
		{
			"non-empty namespace, config w/ pattern",
			"namespace",
			"/aws/ecs/containerinsights/{ClusterName}/performance",
			"{TaskId}",
			"/aws/ecs/containerinsights/test-cluster-name/performance",
			"test-task-id",
		},
		{
			"empty namespace, config w/ pattern",
			"",
			"/aws/ecs/containerinsights/{ClusterName}/performance",
			"{TaskId}",
			"/aws/ecs/containerinsights/test-cluster-name/performance",
			"test-task-id",
		},
		// test case for aws container insight usage
		{
			"empty namespace, config w/ pattern",
			"",
			"/aws/containerinsights/{ClusterName}/performance",
			"{NodeName}",
			"/aws/containerinsights/test-cluster-name/performance",
			"ip-192-168-58-245.ec2.internal",
		},
		// test case for AWS ECS EC2 container insights usage
		{
			"empty namespace, config w/ pattern",
			"",
			"/aws/containerinsights/{ClusterName}/performance",
			"instanceTelemetry/{ContainerInstanceId}",
			"/aws/containerinsights/test-cluster-name/performance",
			"instanceTelemetry/203e0410260d466bab7873bb4f317b4e",
		},
		{
			"empty namespace, config w/ pattern",
			"",
			"/aws/containerinsights/{ClusterName}/performance",
			"{TaskDefinitionFamily}-{TaskId}",
			"/aws/containerinsights/test-cluster-name/performance",
			"test-task-definition-family-test-task-id",
		},
	}

	for i := range rms {
		for _, tc := range testCases {
			t.Run(tc.testName, func(t *testing.T) {
				config := &Config{
					LogGroupName:  tc.configLogGroup,
					LogStreamName: tc.configLogStream,
				}
				logGroup, logStream, success := getLogInfo(rms[i], tc.namespace, config)
				assert.Equal(t, tc.logGroup, logGroup)
				assert.Equal(t, tc.logStream, logStream)
				assert.True(t, success)
			})
		}
	}
}
