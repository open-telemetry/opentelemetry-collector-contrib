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

// assertDimsSorted asserts whether each dimension set within dims
// is sorted alphabetically.
func assertDimsSorted(t *testing.T, dims [][]string) {
	for _, dimSet := range dims {
		if len(dimSet) <= 1 {
			continue
		}
		prevDim := dimSet[0]
		for _, dim := range dimSet[1:] {
			assert.True(t, prevDim <= dim)
			prevDim = dim
		}
	}
}

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

func TestCreateMetricKey(t *testing.T) {
	testCases := []struct {
		testName    string
		labels      map[string]string
		params      map[string]string
		expectedKey string
	}{
		{
			"single label w/o params",
			map[string]string{
				"a": "A",
			},
			nil,
			"a:A",
		},
		{
			"single label w/ params",
			map[string]string{
				"a": "A",
			},
			map[string]string{
				"param1": "foo",
			},
			"a:A,param1:foo",
		},
		{
			"multiple labels w/o params",
			map[string]string{
				"b": "B",
				"a": "A",
				"c": "C",
			},
			nil,
			"a:A,b:B,c:C",
		},
		{
			"multiple labels w/ params",
			map[string]string{
				"b": "B",
				"a": "A",
				"c": "C",
			},
			map[string]string{
				"param1": "foo",
				"bar":    "car",
				"apple":  "banana",
			},
			"a:A,apple:banana,b:B,bar:car,c:C,param1:foo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			key := createMetricKey(tc.labels, tc.params)
			assert.Equal(t, tc.expectedKey, key)
		})
	}
}

func TestDedupDimensions(t *testing.T) {
	testCases := []struct {
		testName   string
		dimensions [][]string
		deduped    [][]string
	}{
		{
			"single dimension",
			[][]string{{"dim1"}},
			[][]string{{"dim1"}},
		},
		{
			"multiple dimensions w/o duplicates",
			[][]string{{"dim1"}, {"dim2"}, {"dim1", "dim2"}},
			[][]string{{"dim1"}, {"dim2"}, {"dim1", "dim2"}},
		},
		{
			"multiple dimensions w/ duplicates",
			[][]string{{"dim1"}, {"dim2"}, {"dim1", "dim2"}, {"dim1", "dim2"}},
			[][]string{{"dim1"}, {"dim2"}, {"dim1", "dim2"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			dedupedDims := dedupDimensions(tc.dimensions)
			assertDimsEqual(t, tc.deduped, dedupedDims)
		})
	}
}

func TestDimensionRollup(t *testing.T) {
	testCases := []struct {
		testName              string
		dimensionRollupOption string
		labels                map[string]string
		expected              [][]string
	}{
		{
			"no rollup w/o instrumentation library name",
			"",
			map[string]string{
				"a": "A",
				"b": "B",
				"c": "C",
			},
			nil,
		},
		{
			"no rollup w/ instrumentation library name",
			"",
			map[string]string{
				"a":                   "A",
				"b":                   "B",
				"c":                   "C",
				(OTellibDimensionKey): "cloudwatch-otel",
			},
			nil,
		},
		{
			"single dim w/o instrumentation library name",
			SingleDimensionRollupOnly,
			map[string]string{
				"a": "A",
				"b": "B",
				"c": "C",
			},
			[][]string{
				{"a"},
				{"b"},
				{"c"},
			},
		},
		{
			"single dim w/ instrumentation library name",
			SingleDimensionRollupOnly,
			map[string]string{
				"a":                   "A",
				"b":                   "B",
				"c":                   "C",
				(OTellibDimensionKey): "cloudwatch-otel",
			},
			[][]string{
				{OTellibDimensionKey, "a"},
				{OTellibDimensionKey, "b"},
				{OTellibDimensionKey, "c"},
			},
		},
		{
			"single dim w/o instrumentation library name and only one label",
			SingleDimensionRollupOnly,
			map[string]string{
				"a": "A",
			},
			[][]string{{"a"}},
		},
		{
			"single dim w/ instrumentation library name and only one label",
			SingleDimensionRollupOnly,
			map[string]string{
				"a":                   "A",
				(OTellibDimensionKey): "cloudwatch-otel",
			},
			[][]string{{OTellibDimensionKey, "a"}},
		},
		{
			"zero + single dim w/o instrumentation library name",
			ZeroAndSingleDimensionRollup,
			map[string]string{
				"a": "A",
				"b": "B",
				"c": "C",
			},
			[][]string{
				{},
				{"a"},
				{"b"},
				{"c"},
			},
		},
		{
			"zero + single dim w/ instrumentation library name",
			ZeroAndSingleDimensionRollup,
			map[string]string{
				"a":                   "A",
				"b":                   "B",
				"c":                   "C",
				"D":                   "d",
				(OTellibDimensionKey): "cloudwatch-otel",
			},
			[][]string{
				{OTellibDimensionKey},
				{OTellibDimensionKey, "a"},
				{OTellibDimensionKey, "b"},
				{OTellibDimensionKey, "c"},
				{OTellibDimensionKey, "D"},
			},
		},
		{
			"zero dim rollup w/o instrumentation library name and no labels",
			ZeroAndSingleDimensionRollup,
			nil,
			nil,
		},
		{
			"zero dim rollup w/ instrumentation library name and no labels",
			ZeroAndSingleDimensionRollup,
			map[string]string{
				(OTellibDimensionKey): "cloudwatch-otel",
			},
			nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			numLabels := len(tc.labels)
			rolledUp := dimensionRollup(tc.dimensionRollupOption, tc.labels)
			// Ensure dimension sets are sorted
			assertDimsSorted(t, rolledUp)
			assertDimsEqual(t, tc.expected, rolledUp)
			// Ensure labels are not changed
			assert.Equal(t, numLabels, len(tc.labels))
		})
	}
}

func BenchmarkCreateMetricKey(b *testing.B) {
	labels := map[string]string{
		"a":                   "A",
		"b":                   "B",
		"c":                   "C",
		(OTellibDimensionKey): "cloudwatch-otel",
	}
	params := map[string]string{
		"param1": "foo",
		"bar":    "car",
		"apple":  "banana",
	}
	m := make(map[string]string)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		key := createMetricKey(labels, params)
		m[key] = "1"
		if _, ok := m[key]; !ok {
			b.FailNow()
		}
	}
}

func BenchmarkDimensionRollup(b *testing.B) {
	labels := map[string]string{
		"a":                   "A",
		"b":                   "B",
		"c":                   "C",
		(OTellibDimensionKey): "cloudwatch-otel",
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		dimensionRollup(ZeroAndSingleDimensionRollup, labels)
	}
}
