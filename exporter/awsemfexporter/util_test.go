// Copyright 2020, OpenTelemetry Authors
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
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

func TestReplacePatternValidTaskId(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskId}"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(2)

	attrMap.UpsertString("aws.ecs.cluster.name", "test-cluster-name")
	attrMap.UpsertString("aws.ecs.task.id", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "test-task-id", s)
}

func TestReplacePatternValidClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(2)

	attrMap.UpsertString("aws.ecs.cluster.name", "test-cluster-name")
	attrMap.UpsertString("aws.ecs.task.id", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
}

func TestReplacePatternMissingAttribute(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(1)

	attrMap.UpsertString("aws.ecs.task.id", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
}

func TestReplacePatternAttrPlaceholderClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(1)

	attrMap.UpsertString("ClusterName", "test-cluster-name")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
}

func TestReplacePatternWrongKey(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{WrongKey}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(1)

	attrMap.UpsertString("ClusterName", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/{WrongKey}/performance", s)
}

func TestReplacePatternNilAttrValue(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InsertNull("ClusterName")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
}

func TestGetNamespace(t *testing.T) {
	defaultMetric := createMetricTestData()
	testCases := []struct {
		testName        string
		metric          consumerdata.MetricsData
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
			consumerdata.MetricsData{
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
			consumerdata.MetricsData{
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
			rms := internaldata.OCToMetrics(tc.metric)
			rm := rms.ResourceMetrics().At(0)
			namespace := getNamespace(&rm, tc.configNamespace)
			assert.Equal(t, tc.namespace, namespace)
		})
	}
}

func TestGetLogInfo(t *testing.T) {
	metric := consumerdata.MetricsData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"aws.ecs.cluster.name": "test-cluster-name",
				"aws.ecs.task.id":      "test-task-id",
			},
		},
	}
	rm := internaldata.OCToMetrics(metric).ResourceMetrics().At(0)

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
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			config := &Config{
				LogGroupName:  tc.configLogGroup,
				LogStreamName: tc.configLogStream,
			}
			logGroup, logStream := getLogInfo(&rm, tc.namespace, config)
			assert.Equal(t, tc.logGroup, logGroup)
			assert.Equal(t, tc.logStream, logStream)
		})
	}
}
