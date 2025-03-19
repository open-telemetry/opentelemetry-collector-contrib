// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

var expectedSpanBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.service.instance.id":"23","Duration":1000000,"EndTimestamp":"2023-04-19T03:04:06.000000006Z","Events.fooEvent.eventMockBar":"bar","Events.fooEvent.eventMockFoo":"foo","Events.fooEvent.time":"2023-04-19T03:04:05.000000006Z","Kind":"SPAN_KIND_CLIENT","Link":"[{\"attribute\":{},\"spanID\":\"\",\"traceID\":\"01020304050607080807060504030200\"}]","Name":"client span","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","Scope.lib-foo":"lib-bar","Scope.name":"io.opentelemetry.rabbitmq-2.7","Scope.version":"1.30.0-alpha","SpanId":"1920212223242526","TraceId":"01020304050607080807060504030201","TraceStatus":2,"TraceStatusDescription":"Test"}`

var (
	expectedLogBody                   = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`
	expectedLogBodyWithEmptyTimestamp = `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`
)

var expectedMetricsEncoded = `{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"idle","system":{"cpu":{"time":440.23}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"interrupt","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"nice","system":{"cpu":{"time":0.14}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"softirq","system":{"cpu":{"time":0.77}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"steal","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"system","system":{"cpu":{"time":24.8}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"user","system":{"cpu":{"time":64.78}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"wait","system":{"cpu":{"time":1.65}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"idle","system":{"cpu":{"time":475.69}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"interrupt","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"nice","system":{"cpu":{"time":0.1}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"softirq","system":{"cpu":{"time":0.57}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"steal","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"system","system":{"cpu":{"time":15.88}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"user","system":{"cpu":{"time":50.09}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"wait","system":{"cpu":{"time":0.95}}}`

func TestEncodeSpan(t *testing.T) {
	encoder, _ := newEncoder(MappingNone)
	td := mockResourceSpans()
	var buf bytes.Buffer
	err := encoder.encodeSpan(
		encodingContext{
			resource: td.ResourceSpans().At(0).Resource(),
			scope:    td.ResourceSpans().At(0).ScopeSpans().At(0).Scope(),
		},
		td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0),
		elasticsearch.Index{}, &buf,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedSpanBody, buf.String())
}

func TestEncodeLog(t *testing.T) {
	t.Run("empty timestamp with observedTimestamp override", func(t *testing.T) {
		encoder, _ := newEncoder(MappingNone)
		td := mockResourceLogs()
		td.ScopeLogs().At(0).LogRecords().At(0).SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)))
		var buf bytes.Buffer
		err := encoder.encodeLog(
			encodingContext{
				resource:          td.Resource(),
				resourceSchemaURL: td.SchemaUrl(),
				scope:             td.ScopeLogs().At(0).Scope(),
				scopeSchemaURL:    td.ScopeLogs().At(0).SchemaUrl(),
			},
			td.ScopeLogs().At(0).LogRecords().At(0),
			elasticsearch.Index{}, &buf,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBody, buf.String())
	})

	t.Run("both timestamp and observedTimestamp empty", func(t *testing.T) {
		encoder, _ := newEncoder(MappingNone)
		td := mockResourceLogs()
		var buf bytes.Buffer
		err := encoder.encodeLog(
			encodingContext{
				resource:          td.Resource(),
				resourceSchemaURL: td.SchemaUrl(),
				scope:             td.ScopeLogs().At(0).Scope(),
				scopeSchemaURL:    td.ScopeLogs().At(0).SchemaUrl(),
			},
			td.ScopeLogs().At(0).LogRecords().At(0),
			elasticsearch.Index{}, &buf,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBodyWithEmptyTimestamp, buf.String())
	})
}

func TestEncodeMetric(t *testing.T) {
	// Prepare metrics to test.
	metrics := createTestMetrics(t)

	// Encode the metrics.
	encoder, _ := newEncoder(MappingECS)
	hasher := newDataPointHasher(MappingECS)

	groupedDataPoints := make(map[uint32][]datapoints.DataPoint)

	var docsBytes [][]byte
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	m := sm.Metrics().At(0)
	dps := m.Sum().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := datapoints.NewNumber(m, dps.At(i))
		dpHash := hasher.hashDataPoint(rm.Resource(), sm.Scope(), dp)
		dataPoints, ok := groupedDataPoints[dpHash]
		if !ok {
			groupedDataPoints[dpHash] = []datapoints.DataPoint{dp}
		} else {
			groupedDataPoints[dpHash] = append(dataPoints, dp)
		}
	}

	for _, dataPoints := range groupedDataPoints {
		var buf bytes.Buffer
		errors := make([]error, 0)
		_, err := encoder.encodeMetrics(
			encodingContext{
				resource:          rm.Resource(),
				resourceSchemaURL: rm.SchemaUrl(),
				scope:             sm.Scope(),
				scopeSchemaURL:    sm.SchemaUrl(),
			},
			dataPoints, &errors, elasticsearch.Index{}, &buf,
		)
		require.Empty(t, errors, err)
		require.NoError(t, err)
		docsBytes = append(docsBytes, buf.Bytes())
	}

	allDocsSorted := docBytesToSortedString(docsBytes)
	assert.Equal(t, expectedMetricsEncoded, allDocsSorted)
}

func docBytesToSortedString(docsBytes [][]byte) string {
	// Convert the byte arrays to strings and sort the docs to make the test deterministic.
	docs := make([]string, len(docsBytes))
	for i, docBytes := range docsBytes {
		docs[i] = string(docBytes)
	}
	sort.Strings(docs)
	allDocsSorted := strings.Join(docs, "\n")
	return allDocsSorted
}

func createTestMetrics(t *testing.T) pmetric.Metrics {
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	metricBytes, err := os.ReadFile("testdata/metrics-cpu.json")
	require.NoError(t, err)
	metrics, err := metricsUnmarshaler.UnmarshalMetrics(metricBytes)
	require.NoError(t, err)
	metrics.MarkReadOnly()
	return metrics
}

func mockResourceSpans() ptrace.Traces {
	traces := ptrace.NewTraces()

	resourceSpans := traces.ResourceSpans().AppendEmpty()
	attr := resourceSpans.Resource().Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_elastic_beanstalk")
	attr.PutStr("deployment.environment", "BETA")
	attr.PutStr("service.instance.id", "23")
	attr.PutStr("service.version", "env-version-1234")

	resourceSpans.Resource().Attributes().PutStr(semconv.AttributeServiceName, "some-service")

	tStart := time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)
	tEnd := time.Date(2023, 4, 19, 3, 4, 6, 6, time.UTC)

	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scopeSpans.Scope().SetName("io.opentelemetry.rabbitmq-2.7")
	scopeSpans.Scope().SetVersion("1.30.0-alpha")
	scopeSpans.Scope().Attributes().PutStr("lib-foo", "lib-bar")

	span := scopeSpans.Spans().AppendEmpty()
	span.SetName("client span")
	span.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
	span.Status().SetCode(2)
	span.Status().SetMessage("Test")
	span.Attributes().PutStr("service.instance.id", "23")
	span.Links().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 0})

	event := span.Events().AppendEmpty()
	event.SetName("fooEvent")
	event.SetTimestamp(pcommon.NewTimestampFromTime(tStart))
	event.Attributes().PutStr("eventMockFoo", "foo")
	event.Attributes().PutStr("eventMockBar", "bar")
	traces.MarkReadOnly()
	return traces
}

func mockResourceLogs() plog.ResourceLogs {
	rl := plog.NewResourceLogs()
	rl.Resource().Attributes().PutStr("key1", "value1")
	l := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	l.Attributes().PutStr("log-attr1", "value1")
	l.Body().SetStr("log-body")
	return rl
}

func TestEncodeAttributes(t *testing.T) {
	t.Parallel()

	logRecord := plog.NewLogRecord()
	err := logRecord.Attributes().FromRaw(map[string]any{
		"s": "baz",
		"o": map[string]any{
			"sub_i": 19,
		},
	})
	require.NoError(t, err)

	tests := map[string]struct {
		mappingMode MappingMode
		want        string
	}{
		"raw": {
			mappingMode: MappingRaw,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "SeverityNumber": 0,
			  "TraceFlags": 0,
			  "o.sub_i": 19,
			  "s": "baz"
			}`,
		},
		"none": {
			mappingMode: MappingNone,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "SeverityNumber": 0,
			  "TraceFlags": 0,
			  "Attributes.o.sub_i": 19,
			  "Attributes.s": "baz"
			}`,
		},
		"ecs": {
			mappingMode: MappingECS,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "agent": {
			    "name": "otlp"
			  },
			  "o": {
			    "sub_i": 19
			  },
			  "s": "baz"
			}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			encoder, err := newEncoder(test.mappingMode)
			require.NoError(t, err)

			var buf bytes.Buffer
			err = encoder.encodeLog(encodingContext{
				resource: pcommon.NewResource(),
				scope:    pcommon.NewInstrumentationScope(),
			}, logRecord, elasticsearch.Index{}, &buf)
			require.NoError(t, err)
			require.JSONEq(t, test.want, buf.String())
		})
	}
}

func TestEncodeSpan_Events(t *testing.T) {
	t.Parallel()

	span := ptrace.NewSpan()
	for i := 0; i < 4; i++ {
		event := span.Events().AppendEmpty()
		event.SetName(fmt.Sprintf("event_%d", i))
	}

	tests := map[string]struct {
		mappingMode MappingMode
		want        string
	}{
		"raw": {
			mappingMode: MappingRaw,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Duration": 0,
			  "EndTimestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "Kind": "SPAN_KIND_UNSPECIFIED",
			  "Link": "[]",
			  "TraceStatus": 0,
			  "event_0.time": "1970-01-01T00:00:00.000000000Z",
			  "event_1.time": "1970-01-01T00:00:00.000000000Z",
			  "event_2.time": "1970-01-01T00:00:00.000000000Z",
			  "event_3.time": "1970-01-01T00:00:00.000000000Z"
			}`,
		},
		"none": {
			mappingMode: MappingNone,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Duration": 0,
			  "EndTimestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "Kind": "SPAN_KIND_UNSPECIFIED",
			  "Link": "[]",
			  "TraceStatus": 0,
			  "Events.event_0.time": "1970-01-01T00:00:00.000000000Z",
			  "Events.event_1.time": "1970-01-01T00:00:00.000000000Z",
			  "Events.event_2.time": "1970-01-01T00:00:00.000000000Z",
			  "Events.event_3.time": "1970-01-01T00:00:00.000000000Z"
			}`,
		},
		"ecs": {
			mappingMode: MappingECS,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Duration": 0,
			  "EndTimestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope": {
			    "name": "",
			    "version": ""
			  },
			  "Kind": "SPAN_KIND_UNSPECIFIED",
			  "Link": "[]",
			  "TraceStatus": 0,
			  "Events": {
			    "event_0": {"time": "1970-01-01T00:00:00.000000000Z"},
			    "event_1": {"time": "1970-01-01T00:00:00.000000000Z"},
			    "event_2": {"time": "1970-01-01T00:00:00.000000000Z"},
			    "event_3": {"time": "1970-01-01T00:00:00.000000000Z"}
			  }
			}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			encoder, err := newEncoder(test.mappingMode)
			require.NoError(t, err)

			var buf bytes.Buffer
			err = encoder.encodeSpan(encodingContext{
				resource: pcommon.NewResource(),
				scope:    pcommon.NewInstrumentationScope(),
			}, span, elasticsearch.Index{}, &buf)
			require.NoError(t, err)
			require.JSONEq(t, test.want, buf.String())
		})
	}
}

func TestEncodeLogECSModeDuplication(t *testing.T) {
	logs := plog.NewLogs()
	resource := logs.ResourceLogs().AppendEmpty().Resource()
	err := resource.Attributes().FromRaw(map[string]any{
		semconv.AttributeServiceName:    "foo.bar",
		semconv.AttributeHostName:       "localhost",
		semconv.AttributeServiceVersion: "1.1.0",
		semconv.AttributeOSType:         "darwin",
		semconv.AttributeOSDescription:  "Mac OS Mojave",
		semconv.AttributeOSName:         "Mac OS X",
		semconv.AttributeOSVersion:      "10.14.1",
	})
	require.NoError(t, err)

	want := `{"@timestamp":"2024-03-12T20:00:41.123456789Z","agent":{"name":"otlp"},"container":{"image":{"tag":["v3.4.0"]}},"event":{"action":"user-password-change"},"host":{"hostname":"localhost","name":"localhost","os":{"full":"Mac OS Mojave","name":"Mac OS X","platform":"darwin","type":"macos","version":"10.14.1"}},"service":{"name":"foo.bar","version":"1.1.0"}}`
	require.NoError(t, err)

	resourceContainerImageTags := resource.Attributes().PutEmptySlice(semconv.AttributeContainerImageTags)
	err = resourceContainerImageTags.FromRaw([]any{"v3.4.0"})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()

	record := plog.NewLogRecord()
	err = record.Attributes().FromRaw(map[string]any{
		"event.name": "user-password-change",
	})
	require.NoError(t, err)
	observedTimestamp := pcommon.Timestamp(1710273641123456789)
	record.SetObservedTimestamp(observedTimestamp)
	logs.MarkReadOnly()

	var buf bytes.Buffer
	encoder, _ := newEncoder(MappingECS)
	err = encoder.encodeLog(
		encodingContext{resource: resource, scope: scope},
		record, elasticsearch.Index{}, &buf,
	)
	require.NoError(t, err)

	assert.Equal(t, want, buf.String())
}

func TestEncodeLogECSMode(t *testing.T) {
	logs := plog.NewLogs()
	resource := logs.ResourceLogs().AppendEmpty().Resource()
	err := resource.Attributes().FromRaw(map[string]any{
		semconv.AttributeServiceName:           "foo.bar",
		semconv.AttributeServiceVersion:        "1.1.0",
		semconv.AttributeServiceInstanceID:     "i-103de39e0a",
		semconv.AttributeTelemetrySDKName:      "opentelemetry",
		semconv.AttributeTelemetrySDKVersion:   "7.9.12",
		semconv.AttributeTelemetrySDKLanguage:  "perl",
		semconv.AttributeCloudProvider:         "gcp",
		semconv.AttributeCloudAccountID:        "19347013",
		semconv.AttributeCloudRegion:           "us-west-1",
		semconv.AttributeCloudAvailabilityZone: "us-west-1b",
		semconv.AttributeCloudPlatform:         "gke",
		semconv.AttributeContainerName:         "happy-seger",
		semconv.AttributeContainerID:           "e69cc5d3dda",
		semconv.AttributeContainerImageName:    "my-app",
		semconv.AttributeContainerRuntime:      "docker",
		semconv.AttributeHostName:              "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		semconv.AttributeHostID:                "i-103de39e0a",
		semconv.AttributeHostType:              "t2.medium",
		semconv.AttributeHostArch:              "x86_64",
		semconv.AttributeProcessPID:            9833,
		semconv.AttributeProcessCommandLine:    "/usr/bin/ssh -l user 10.0.0.16",
		semconv.AttributeProcessExecutablePath: "/usr/bin/ssh",
		semconv.AttributeProcessRuntimeName:    "OpenJDK Runtime Environment",
		semconv.AttributeProcessRuntimeVersion: "14.0.2",
		semconv.AttributeOSType:                "darwin",
		semconv.AttributeOSDescription:         "Mac OS Mojave",
		semconv.AttributeOSName:                "Mac OS X",
		semconv.AttributeOSVersion:             "10.14.1",
		semconv.AttributeDeviceID:              "00000000-54b3-e7c7-0000-000046bffd97",
		semconv.AttributeDeviceModelIdentifier: "SM-G920F",
		semconv.AttributeDeviceModelName:       "Samsung Galaxy S6",
		semconv.AttributeDeviceManufacturer:    "Samsung",
		"k8s.namespace.name":                   "default",
		"k8s.node.name":                        "node-1",
		"k8s.pod.name":                         "opentelemetry-pod-autoconf",
		"k8s.pod.uid":                          "275ecb36-5aa8-4c2a-9c47-d8bb681b9aff",
		"k8s.deployment.name":                  "coredns",
		semconv.AttributeK8SJobName:            "job.name",
		semconv.AttributeK8SCronJobName:        "cronjob.name",
		semconv.AttributeK8SStatefulSetName:    "statefulset.name",
		semconv.AttributeK8SReplicaSetName:     "replicaset.name",
		semconv.AttributeK8SDaemonSetName:      "daemonset.name",
		semconv.AttributeK8SContainerName:      "container.name",
		semconv.AttributeK8SClusterName:        "cluster.name",
	})
	require.NoError(t, err)

	resourceContainerImageTags := resource.Attributes().PutEmptySlice(semconv.AttributeContainerImageTags)
	err = resourceContainerImageTags.FromRaw([]any{"v3.4.0"})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()

	record := plog.NewLogRecord()
	err = record.Attributes().FromRaw(map[string]any{
		"event.name": "user-password-change",
	})
	require.NoError(t, err)
	observedTimestamp := pcommon.Timestamp(1710273641123456789)
	record.SetObservedTimestamp(observedTimestamp)
	logs.MarkReadOnly()

	var buf bytes.Buffer
	encoder, _ := newEncoder(MappingECS)
	err = encoder.encodeLog(encodingContext{
		resource: resource,
		scope:    scope,
	}, record, elasticsearch.Index{}, &buf)
	require.NoError(t, err)

	require.JSONEq(t, `{
		"@timestamp": "2024-03-12T20:00:41.123456789Z",
		"agent": {
		  "name": "opentelemetry/perl",
		  "version": "7.9.12"
		},
		"cloud": {
		  "provider": "gcp",
		  "account": {"id": "19347013"},
		  "region": "us-west-1",
		  "availability_zone": "us-west-1b",
		  "service": {"name": "gke"}
		},
		"container": {
		  "name": "happy-seger",
		  "id": "e69cc5d3dda",
		  "image": {
		    "name": "my-app",
		    "tag": ["v3.4.0"]
		  },
		  "runtime": "docker"
		},
		"host": {
		  "hostname": "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		  "name": "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		  "id": "i-103de39e0a",
		  "type": "t2.medium",
		  "architecture": "x86_64",
		  "os": {
		    "platform": "darwin",
		    "full": "Mac OS Mojave",
		    "name": "Mac OS X",
		    "version": "10.14.1",
		    "type": "macos"
		  }
		},
		"process": {
		  "pid": 9833,
		  "command_line": "/usr/bin/ssh -l user 10.0.0.16",
		  "executable": "/usr/bin/ssh"
		},
		"service": {
		  "name": "foo.bar",
		  "version": "1.1.0",
		  "node": {"name": "i-103de39e0a"},
		  "runtime": {
		    "name": "OpenJDK Runtime Environment",
		    "version": "14.0.2"
		  }
		},
		"device": {
		  "id": "00000000-54b3-e7c7-0000-000046bffd97",
		  "model": {
		    "identifier": "SM-G920F",
		    "name": "Samsung Galaxy S6"
		  },
		  "manufacturer": "Samsung"
		},
		"event": {"action": "user-password-change"},
		"kubernetes": {
		  "namespace": "default",
		  "node": {"name": "node-1"},
		  "pod": {
		    "name": "opentelemetry-pod-autoconf",
		    "uid": "275ecb36-5aa8-4c2a-9c47-d8bb681b9aff"
		  },
		  "deployment": {"name": "coredns"},
		  "job": {"name": "job.name"},
		  "cronjob": {"name": "cronjob.name"},
		  "statefulset": {"name": "statefulset.name"},
		  "replicaset": {"name": "replicaset.name"},
		  "daemonset": {"name": "daemonset.name"},
		  "container": {"name": "container.name"}
		},
		"orchestrator": {"cluster": {"name": "cluster.name"}}
	}`, buf.String())
}

func TestEncodeLogECSModeAgentName(t *testing.T) {
	tests := map[string]struct {
		telemetrySdkName     string
		telemetrySdkLanguage string
		telemetryDistroName  string

		expectedAgentName           string
		expectedServiceLanguageName string
	}{
		"none_set": {
			expectedAgentName:           "otlp",
			expectedServiceLanguageName: "unknown",
		},
		"name_set": {
			telemetrySdkName:            "opentelemetry",
			expectedAgentName:           "opentelemetry",
			expectedServiceLanguageName: "unknown",
		},
		"language_set": {
			telemetrySdkLanguage:        "java",
			expectedAgentName:           "otlp/java",
			expectedServiceLanguageName: "java",
		},
		"distro_set": {
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "otlp/unknown/parts-unlimited-java",
			expectedServiceLanguageName: "unknown",
		},
		"name_language_set": {
			telemetrySdkName:            "opentelemetry",
			telemetrySdkLanguage:        "java",
			expectedAgentName:           "opentelemetry/java",
			expectedServiceLanguageName: "java",
		},
		"name_distro_set": {
			telemetrySdkName:            "opentelemetry",
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "opentelemetry/unknown/parts-unlimited-java",
			expectedServiceLanguageName: "unknown",
		},
		"language_distro_set": {
			telemetrySdkLanguage:        "java",
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "otlp/java/parts-unlimited-java",
			expectedServiceLanguageName: "java",
		},
		"name_language_distro_set": {
			telemetrySdkName:            "opentelemetry",
			telemetrySdkLanguage:        "java",
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "opentelemetry/java/parts-unlimited-java",
			expectedServiceLanguageName: "java",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs := plog.NewLogs()
			resource := logs.ResourceLogs().AppendEmpty().Resource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.telemetrySdkName != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetrySDKName, test.telemetrySdkName)
			}
			if test.telemetrySdkLanguage != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetrySDKLanguage, test.telemetrySdkLanguage)
			}
			if test.telemetryDistroName != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetryDistroName, test.telemetryDistroName)
			}

			timestamp := pcommon.Timestamp(1710373859123456789)
			record.SetTimestamp(timestamp)
			logs.MarkReadOnly()

			var buf bytes.Buffer
			encoder, _ := newEncoder(MappingECS)
			err := encoder.encodeLog(
				encodingContext{resource: resource, scope: scope},
				record, elasticsearch.Index{}, &buf,
			)
			require.NoError(t, err)
			require.JSONEq(t, fmt.Sprintf(`{
				"@timestamp": "2024-03-13T23:50:59.123456789Z",
				"agent": {"name": %q}
			}`, test.expectedAgentName), buf.String())
		})
	}
}

func TestEncodeLogECSModeAgentVersion(t *testing.T) {
	tests := map[string]struct {
		telemetryDistroVersion string
		telemetrySdkVersion    string
		expectedAgentVersion   string
	}{
		"none_set": {
			expectedAgentVersion: "",
		},
		"distro_version_set": {
			telemetryDistroVersion: "7.9.2",
			expectedAgentVersion:   "7.9.2",
		},
		"sdk_version_set": {
			telemetrySdkVersion:  "8.10.3",
			expectedAgentVersion: "8.10.3",
		},
		"both_set": {
			telemetryDistroVersion: "7.9.2",
			telemetrySdkVersion:    "8.10.3",
			expectedAgentVersion:   "7.9.2",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs := plog.NewLogs()
			resource := logs.ResourceLogs().AppendEmpty().Resource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.telemetryDistroVersion != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetryDistroVersion, test.telemetryDistroVersion)
			}
			if test.telemetrySdkVersion != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetrySDKVersion, test.telemetrySdkVersion)
			}

			timestamp := pcommon.Timestamp(1710373859123456789)
			record.SetTimestamp(timestamp)
			logs.MarkReadOnly()

			var buf bytes.Buffer
			encoder, _ := newEncoder(MappingECS)
			err := encoder.encodeLog(
				encodingContext{resource: resource, scope: scope},
				record, elasticsearch.Index{}, &buf,
			)
			require.NoError(t, err)

			if test.expectedAgentVersion == "" {
				require.JSONEq(t, `{
					"@timestamp": "2024-03-13T23:50:59.123456789Z",
					"agent": {"name": "otlp"}
				}`, buf.String())
			} else {
				require.JSONEq(t, fmt.Sprintf(`{
					"@timestamp": "2024-03-13T23:50:59.123456789Z",
					"agent": {"name": "otlp", "version": %q}
				}`, test.expectedAgentVersion), buf.String())
			}
		})
	}
}

func TestEncodeLogECSModeHostOSType(t *testing.T) {
	tests := map[string]struct {
		osType string
		osName string

		expectedHostOsName     string
		expectedHostOsType     string
		expectedHostOsPlatform string
	}{
		"none_set": {
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "", // should not be set
			expectedHostOsPlatform: "", // should not be set
		},
		"type_windows": {
			osType:                 "windows",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "windows",
			expectedHostOsPlatform: "windows",
		},
		"type_linux": {
			osType:                 "linux",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "linux",
			expectedHostOsPlatform: "linux",
		},
		"type_darwin": {
			osType:                 "darwin",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "macos",
			expectedHostOsPlatform: "darwin",
		},
		"type_aix": {
			osType:                 "aix",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "aix",
		},
		"type_hpux": {
			osType:                 "hpux",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "hpux",
		},
		"type_solaris": {
			osType:                 "solaris",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "solaris",
		},
		"type_unknown": {
			osType:                 "unknown",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "", // should not be set
			expectedHostOsPlatform: "unknown",
		},
		"name_android": {
			osName:                 "Android",
			expectedHostOsName:     "Android",
			expectedHostOsType:     "android",
			expectedHostOsPlatform: "", // should not be set
		},
		"name_ios": {
			osName:                 "iOS",
			expectedHostOsName:     "iOS",
			expectedHostOsType:     "ios",
			expectedHostOsPlatform: "", // should not be set
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs := plog.NewLogs()
			resource := logs.ResourceLogs().AppendEmpty().Resource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.osType != "" {
				resource.Attributes().PutStr(semconv.AttributeOSType, test.osType)
			}
			if test.osName != "" {
				resource.Attributes().PutStr(semconv.AttributeOSName, test.osName)
			}

			timestamp := pcommon.Timestamp(1710373859123456789)
			record.SetTimestamp(timestamp)
			logs.MarkReadOnly()

			var buf bytes.Buffer
			encoder, _ := newEncoder(MappingECS)
			err := encoder.encodeLog(
				encodingContext{resource: resource, scope: scope},
				record, elasticsearch.Index{}, &buf,
			)
			require.NoError(t, err)

			expectedJSON := `{"@timestamp":"2024-03-13T23:50:59.123456789Z", "agent":{"name":"otlp"}`
			if test.expectedHostOsName != "" ||
				test.expectedHostOsPlatform != "" ||
				test.expectedHostOsType != "" {
				expectedJSON += `, "host":{"os":{`

				first := true
				maybeAdd := func(k, v string) {
					if v != "" {
						if first {
							first = false
						} else {
							expectedJSON += ","
						}
						expectedJSON += fmt.Sprintf("%q:%q", k, v)
					}
				}
				maybeAdd("name", test.expectedHostOsName)
				maybeAdd("type", test.expectedHostOsType)
				maybeAdd("platform", test.expectedHostOsPlatform)
				expectedJSON += "}}"
			}
			expectedJSON += "}"
			require.JSONEq(t, expectedJSON, buf.String())
		})
	}
}

func TestEncodeLogECSModeTimestamps(t *testing.T) {
	tests := map[string]struct {
		timeUnixNano         int64
		observedTimeUnixNano int64
		expectedTimestamp    string
	}{
		"only_observed_set": {
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    "2024-03-12T20:00:41.123456789Z",
		},
		"both_set": {
			timeUnixNano:         1710273639345678901,
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    "2024-03-12T20:00:39.345678901Z",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := pcommon.NewResource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.timeUnixNano > 0 {
				record.SetTimestamp(pcommon.Timestamp(test.timeUnixNano))
			}
			if test.observedTimeUnixNano > 0 {
				record.SetObservedTimestamp(pcommon.Timestamp(test.observedTimeUnixNano))
			}

			var buf bytes.Buffer
			encoder, _ := newEncoder(MappingECS)
			err := encoder.encodeLog(
				encodingContext{resource: resource, scope: scope},
				record, elasticsearch.Index{}, &buf,
			)
			require.NoError(t, err)

			require.JSONEq(t, fmt.Sprintf(
				`{"@timestamp":%q,"agent":{"name":"otlp"}}`, test.expectedTimestamp,
			), buf.String())
		})
	}
}

func TestMapLogAttributesToECS(t *testing.T) {
	tests := map[string]struct {
		attrs         func() pcommon.Map
		conversionMap map[string]string
		preserveMap   map[string]bool
		expectedDoc   func() objmodel.Document
	}{
		"no_attrs": {
			attrs: pcommon.NewMap,
			conversionMap: map[string]string{
				"foo.bar": "baz",
			},
			expectedDoc: func() objmodel.Document {
				return objmodel.Document{}
			},
		},
		"no_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				return d
			},
		},
		"empty_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]string{},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				return d
			},
		},
		"all_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]string{
				"foo.bar": "bar.qux",
				"qux":     "foo",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddInt("foo", 17)
				return d
			},
		},
		"some_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]string{
				"foo.bar": "bar.qux",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddInt("qux", 17)
				return d
			},
		},
		"no_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]string{
				"baz": "qux",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				d.AddInt("qux", 17)
				return d
			},
		},
		"extra_keys_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]string{
				"foo.bar": "bar.qux",
				"qux":     "foo",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				return d
			},
		},
		"preserve_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]string{
				"foo.bar": "bar.qux",
				"qux":     "foo",
			}, preserveMap: map[string]bool{
				"foo.bar": true,
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddString("foo.bar", "baz")
				return d
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var doc objmodel.Document
			encodeAttributesECSMode(&doc, test.attrs(), test.conversionMap, test.preserveMap)

			expectedDoc := test.expectedDoc()
			require.Equal(t, expectedDoc, doc)
		})
	}
}

// JSON serializable structs for OTel test convenience
type OTelRecord struct {
	TraceID                OTelTraceID          `json:"trace_id"`
	SpanID                 OTelSpanID           `json:"span_id"`
	SeverityNumber         int32                `json:"severity_number"`
	SeverityText           string               `json:"severity_text"`
	EventName              string               `json:"event_name"`
	Attributes             map[string]any       `json:"attributes"`
	DroppedAttributesCount uint32               `json:"dropped_attributes_count"`
	Scope                  OTelScope            `json:"scope"`
	Resource               OTelResource         `json:"resource"`
	Datastream             OTelRecordDatastream `json:"data_stream"`
}

type OTelRecordDatastream struct {
	Dataset   string `json:"dataset"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

type OTelScope struct {
	Name                   string         `json:"name"`
	Version                string         `json:"version"`
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	SchemaURL              string         `json:"schema_url"`
}

type OTelResource struct {
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	SchemaURL              string         `json:"schema_url"`
}

type OTelSpanID pcommon.SpanID

func (o OTelSpanID) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (o *OTelSpanID) UnmarshalJSON(data []byte) error {
	b, err := decodeOTelID(data)
	if err != nil {
		return err
	}
	copy(o[:], b)
	return nil
}

type OTelTraceID pcommon.TraceID

func (o OTelTraceID) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (o *OTelTraceID) UnmarshalJSON(data []byte) error {
	b, err := decodeOTelID(data)
	if err != nil {
		return err
	}
	copy(o[:], b)
	return nil
}

func decodeOTelID(data []byte) ([]byte, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return hex.DecodeString(s)
}

func TestEncodeLogOtelMode(t *testing.T) {
	randomString := strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789", 10)
	maxLenNamespace := maxDataStreamBytes - len(disallowedNamespaceRunes)
	maxLenDataset := maxDataStreamBytes - len(disallowedDatasetRunes) - len(".otel")

	tests := []struct {
		name   string
		rec    OTelRecord
		wantFn func(OTelRecord) OTelRecord // Allows each test to customized the expectations from the original test record data
	}{
		{
			name: "default", // Expecting default data_stream values
			rec:  buildOTelRecordTestData(t, nil),
			wantFn: func(or OTelRecord) OTelRecord {
				return assignDatastreamData(or)
			},
		},
		{
			name: "custom dataset",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "custom"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				// Datastream attributes are expected to be deleted from under the attributes
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "custom.otel")
			},
		},
		{
			name: "custom dataset with otel suffix",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "custom.otel"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "custom.otel.otel")
			},
		},
		{
			name: "custom dataset/namespace",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "customds"
				or.Attributes["data_stream.namespace"] = "customns"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "customds.otel", "customns")
			},
		},
		{
			name: "dataset attributes priority",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "first"
				or.Scope.Attributes["data_stream.dataset"] = "second"
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "first.otel")
			},
		},
		{
			name: "dataset scope attribute priority",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Scope.Attributes["data_stream.dataset"] = "second"
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "second.otel")
			},
		},
		{
			name: "dataset resource attribute priority",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "third.otel")
			},
		},
		{
			name: "sanitize dataset/namespace",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = disallowedDatasetRunes + randomString
				or.Attributes["data_stream.namespace"] = disallowedNamespaceRunes + randomString
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				ds := strings.Repeat("_", len(disallowedDatasetRunes)) + randomString[:maxLenDataset] + ".otel"
				ns := strings.Repeat("_", len(disallowedNamespaceRunes)) + randomString[:maxLenNamespace]
				return assignDatastreamData(or, "", ds, ns)
			},
		},
		{
			name: "event_name from attributes.event.name",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["event.name"] = "foo"
				or.EventName = ""
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				or.EventName = "foo"
				return assignDatastreamData(or)
			},
		},
		{
			name: "event_name takes precedent over attributes.event.name",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["event.name"] = "foo"
				or.EventName = "bar"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				or.EventName = "bar"
				return assignDatastreamData(or)
			},
		},
	}

	encoder, _ := newEncoder(MappingOTel)

	for _, tc := range tests {
		record, scope, resource := createTestOTelLogRecord(t, tc.rec)
		router := newDocumentRouter(MappingOTel, "", &Config{})

		idx, err := router.routeLogRecord(resource, scope, record.Attributes())
		require.NoError(t, err)

		var buf bytes.Buffer
		err = encoder.encodeLog(
			encodingContext{
				resource:          resource,
				resourceSchemaURL: tc.rec.Resource.SchemaURL,
				scope:             scope,
				scopeSchemaURL:    tc.rec.Scope.SchemaURL,
			},
			record, idx, &buf,
		)
		require.NoError(t, err)

		want := tc.rec
		if tc.wantFn != nil {
			want = tc.wantFn(want)
		}

		var got OTelRecord
		err = json.Unmarshal(buf.Bytes(), &got)

		require.NoError(t, err)

		assert.Equal(t, want, got)
	}
}

// helper function that creates the OTel LogRecord from the test structure
func createTestOTelLogRecord(t *testing.T, rec OTelRecord) (plog.LogRecord, pcommon.InstrumentationScope, pcommon.Resource) {
	record := plog.NewLogRecord()

	record.SetTraceID(pcommon.TraceID(rec.TraceID))
	record.SetSpanID(pcommon.SpanID(rec.SpanID))
	record.SetSeverityNumber(plog.SeverityNumber(rec.SeverityNumber))
	record.SetSeverityText(rec.SeverityText)
	record.SetDroppedAttributesCount(rec.DroppedAttributesCount)
	record.SetEventName(rec.EventName)

	err := record.Attributes().FromRaw(rec.Attributes)
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()
	scope.SetName(rec.Scope.Name)
	scope.SetVersion(rec.Scope.Version)
	scope.SetDroppedAttributesCount(rec.Scope.DroppedAttributesCount)
	err = scope.Attributes().FromRaw(rec.Scope.Attributes)
	require.NoError(t, err)

	resource := pcommon.NewResource()
	resource.SetDroppedAttributesCount(rec.Resource.DroppedAttributesCount)
	err = resource.Attributes().FromRaw(rec.Resource.Attributes)
	require.NoError(t, err)

	return record, scope, resource
}

func buildOTelRecordTestData(t *testing.T, fn func(OTelRecord) OTelRecord) OTelRecord {
	s := `{
    "@timestamp": "2024-03-12T20:00:41.123456780Z",
    "attributes": {
        "event.name": "user-password-change",
        "foo.some": "bar"
    },
    "event_name": "user-password-change",
    "dropped_attributes_count": 1,
    "observed_timestamp": "2024-03-12T20:00:41.123456789Z",
    "resource": {
        "attributes": {
            "host.name": "lebuntu",
            "host.os.type": "linux"
        },
        "dropped_attributes_count": 2,
        "schema_url": "https://opentelemetry.io/schemas/1.6.0"
    },
    "scope": {
        "attributes": {
            "attr.num": 1234,
            "attr.str": "val1"
        },
        "dropped_attributes_count": 2,
        "name": "foobar",
        "schema_url": "https://opentelemetry.io/schemas/1.6.1",
        "version": "42"
    },
    "severity_number": 17,
    "severity_text": "ERROR",
    "span_id": "0102030405060708",
    "trace_id": "01020304050607080900010203040506"
}`

	var record OTelRecord
	err := json.Unmarshal([]byte(s), &record)
	assert.NoError(t, err)
	if fn != nil {
		record = fn(record)
	}
	return record
}

func deleteDatasetAttributes(or OTelRecord) {
	deleteDatasetAttributesFromMap(or.Attributes)
	deleteDatasetAttributesFromMap(or.Scope.Attributes)
	deleteDatasetAttributesFromMap(or.Resource.Attributes)
}

func deleteDatasetAttributesFromMap(m map[string]any) {
	delete(m, "data_stream.dataset")
	delete(m, "data_stream.namespace")
	delete(m, "data_stream.type")
}

func assignDatastreamData(or OTelRecord, a ...string) OTelRecord {
	r := OTelRecordDatastream{
		Dataset:   "generic.otel",
		Namespace: "default",
		Type:      "logs",
	}

	if len(a) > 0 && a[0] != "" {
		r.Type = a[0]
	}
	if len(a) > 1 && a[1] != "" {
		r.Dataset = a[1]
	}
	if len(a) > 2 && a[2] != "" {
		r.Namespace = a[2]
	}

	or.Datastream = r

	return or
}

func TestEncodeLogScalarObjectConflict(t *testing.T) {
	// If there is an attribute named "foo", and another called "foo.bar",
	// then "foo" will be renamed to "foo.value".
	encoder, _ := newEncoder(MappingNone)
	td := mockResourceLogs()
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo", "scalar")
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo.bar", "baz")
	var buf bytes.Buffer
	err := encoder.encodeLog(
		encodingContext{resource: td.Resource(), scope: td.ScopeLogs().At(0).Scope()},
		td.ScopeLogs().At(0).LogRecords().At(0), elasticsearch.Index{}, &buf,
	)
	assert.NoError(t, err)

	encoded := buf.Bytes()
	assert.True(t, gjson.ValidBytes(encoded))
	assert.False(t, gjson.GetBytes(encoded, "Attributes\\.foo").Exists())
	fooValue := gjson.GetBytes(encoded, "Attributes\\.foo\\.value")
	fooBar := gjson.GetBytes(encoded, "Attributes\\.foo\\.bar")
	assert.Equal(t, "scalar", fooValue.Str)
	assert.Equal(t, "baz", fooBar.Str)

	// If there is an attribute named "foo.value", then "foo" would be omitted rather than renamed.
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo.value", "foovalue")
	buf = bytes.Buffer{}
	err = encoder.encodeLog(
		encodingContext{resource: td.Resource(), scope: td.ScopeLogs().At(0).Scope()},
		td.ScopeLogs().At(0).LogRecords().At(0), elasticsearch.Index{}, &buf,
	)
	assert.NoError(t, err)

	encoded = buf.Bytes()
	assert.False(t, gjson.GetBytes(encoded, "Attributes\\.foo").Exists())
	fooValue = gjson.GetBytes(encoded, "Attributes\\.foo\\.value")
	assert.Equal(t, "foovalue", fooValue.Str)
}

func TestEncodeLogBodyMapMode(t *testing.T) {
	// craft a log record with a body map
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecords := scopeLogs.LogRecords()
	observedTimestamp := pcommon.Timestamp(time.Now().UnixNano())

	logRecord := logRecords.AppendEmpty()
	logRecord.SetObservedTimestamp(observedTimestamp)

	bodyMap := pcommon.NewMap()
	bodyMap.PutStr("@timestamp", "2024-03-12T20:00:41.123456789Z")
	bodyMap.PutInt("id", 1)
	bodyMap.PutStr("key", "value")
	bodyMap.PutStr("key.a", "a")
	bodyMap.PutStr("key.a.b", "b")
	bodyMap.PutDouble("pi", 3.14)
	bodyMap.CopyTo(logRecord.Body().SetEmptyMap())

	encoder, _ := newEncoder(MappingBodyMap)
	var buf bytes.Buffer
	err := encoder.encodeLog(
		encodingContext{resource: resourceLogs.Resource(), scope: scopeLogs.Scope()},
		logRecord, elasticsearch.Index{}, &buf,
	)
	require.NoError(t, err)

	require.JSONEq(t, `{
		"@timestamp":                 "2024-03-12T20:00:41.123456789Z",
		"id":                         1,
		"key":                        "value",
		"key.a":                      "a",
		"key.a.b":                    "b",
		"pi":                         3.14
	}`, buf.String())

	// invalid body map
	logRecord.Body().SetEmptySlice()
	err = encoder.encodeLog(
		encodingContext{resource: resourceLogs.Resource(), scope: scopeLogs.Scope()},
		logRecord, elasticsearch.Index{}, &buf,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidTypeForBodyMapMode)
}
