// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"testing"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

func TestReplacePatternValidTaskId(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskId}"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("aws.ecs.cluster.name", "test-cluster-name")
	attrMap.PutStr("aws.ecs.task.id", "test-task-id")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "test-task-id", s)
	assert.True(t, success)
}

func TestReplacePatternValidServiceName(t *testing.T) {
	logger := zap.NewNop()

	input := "{ServiceName}"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("service.name", "some-test-service")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "some-test-service", s)
	assert.True(t, success)
}

func TestReplacePatternValidClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("aws.ecs.cluster.name", "test-cluster-name")
	attrMap.PutStr("aws.ecs.task.id", "test-task-id")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
	assert.True(t, success)
}

func TestReplacePatternMissingAttribute(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("aws.ecs.task.id", "test-task-id")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternValidPodName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("aws.eks.cluster.name", "test-cluster-name")
	attrMap.PutStr("PodName", "test-pod-001")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/test-pod-001/performance", s)
	assert.True(t, success)
}

func TestReplacePatternValidPod(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("aws.eks.cluster.name", "test-cluster-name")
	attrMap.PutStr("pod", "test-pod-001")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/test-pod-001/performance", s)
	assert.True(t, success)
}

func TestReplacePatternMissingPodName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/eks/containerinsights/{PodName}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("aws.eks.cluster.name", "test-cluster-name")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/eks/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternAttrPlaceholderClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("ClusterName", "test-cluster-name")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
	assert.True(t, success)
}

func TestReplacePatternWrongKey(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{WrongKey}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("ClusterName", "test-task-id")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/{WrongKey}/performance", s)
	assert.True(t, success)
}

func TestReplacePatternNilAttrValue(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pcommon.NewMap()
	attrMap.PutEmpty("ClusterName")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
	assert.False(t, success)
}

func TestReplacePatternValidTaskDefinitionFamily(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskDefinitionFamily}"

	attrMap := pcommon.NewMap()
	attrMap.PutStr("aws.ecs.cluster.name", "test-cluster-name")
	attrMap.PutStr("aws.ecs.task.family", "test-task-definition-family")

	s, success := replacePatterns(input, attrMaptoStringMap(attrMap), logger)

	assert.Equal(t, "test-task-definition-family", s)
	assert.True(t, success)
}

func TestGetNamespace(t *testing.T) {
	defaultMetric := createMetricTestData()
	testCases := []struct {
		testName        string
		metric          *agentmetricspb.ExportMetricsServiceRequest
		configNamespace string
		namespace       string
	}{
		{
			"non-empty namespace",
			defaultMetric,
			"namespace",
			"namespace",
		},
		{
			"empty namespace",
			defaultMetric,
			"",
			"myServiceNS/myServiceName",
		},
		{
			"empty namespace, no service namespace",
			&agentmetricspb.ExportMetricsServiceRequest{
				Resource: &resourcepb.Resource{
					Labels: map[string]string{
						conventions.AttributeServiceName: "myServiceName",
					},
				},
			},
			"",
			"myServiceName",
		},
		{
			"empty namespace, no service name",
			&agentmetricspb.ExportMetricsServiceRequest{
				Resource: &resourcepb.Resource{
					Labels: map[string]string{
						conventions.AttributeServiceNamespace: "myServiceNS",
					},
				},
			},
			"",
			"myServiceNS",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			rms := internaldata.OCToMetrics(tc.metric.Node, tc.metric.Resource, tc.metric.Metrics)
			rm := rms.ResourceMetrics().At(0)
			namespace := getNamespace(rm, tc.configNamespace)
			assert.Equal(t, tc.namespace, namespace)
		})
	}
}

func TestGetLogInfo(t *testing.T) {
	metrics := []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Node: &commonpb.Node{
				ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
				LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
			},
			Resource: &resourcepb.Resource{
				Labels: map[string]string{
					"aws.ecs.cluster.name":          "test-cluster-name",
					"aws.ecs.task.id":               "test-task-id",
					"k8s.node.name":                 "ip-192-168-58-245.ec2.internal",
					"aws.ecs.container.instance.id": "203e0410260d466bab7873bb4f317b4e",
					"aws.ecs.task.family":           "test-task-definition-family",
				},
			},
		},
		{
			Node: &commonpb.Node{
				ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
				LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
			},
			Resource: &resourcepb.Resource{
				Labels: map[string]string{
					"ClusterName":          "test-cluster-name",
					"TaskId":               "test-task-id",
					"NodeName":             "ip-192-168-58-245.ec2.internal",
					"ContainerInstanceId":  "203e0410260d466bab7873bb4f317b4e",
					"TaskDefinitionFamily": "test-task-definition-family",
				},
			},
		},
	}

	var rms []pmetric.ResourceMetrics
	for _, md := range metrics {
		rms = append(rms, internaldata.OCToMetrics(md.Node, md.Resource, md.Metrics).ResourceMetrics().At(0))
	}

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
