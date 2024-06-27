// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

var expectedSpanBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.service.instance.id":"23","Duration":1000000,"EndTimestamp":"2023-04-19T03:04:06.000000006Z","Events.fooEvent.evnetMockBar":"bar","Events.fooEvent.evnetMockFoo":"foo","Events.fooEvent.time":"2023-04-19T03:04:05.000000006Z","Kind":"SPAN_KIND_CLIENT","Link":"[{\"attribute\":{},\"spanID\":\"\",\"traceID\":\"01020304050607080807060504030200\"}]","Name":"client span","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","Scope.lib-foo":"lib-bar","Scope.name":"io.opentelemetry.rabbitmq-2.7","Scope.version":"1.30.0-alpha","SpanId":"1920212223242526","TraceId":"01020304050607080807060504030201","TraceStatus":2,"TraceStatusDescription":"Test"}`

var expectedLogBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`

var expectedMetricsEncoded = `{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"idle","system":{"cpu":{"time":440.23}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"interrupt","system":{"cpu":{"time":0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"nice","system":{"cpu":{"time":0.14}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"softirq","system":{"cpu":{"time":0.77}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"steal","system":{"cpu":{"time":0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"system","system":{"cpu":{"time":24.8}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"user","system":{"cpu":{"time":64.78}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"name":"my-host"},"os":{"type":"linux"},"state":"wait","system":{"cpu":{"time":1.65}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"idle","system":{"cpu":{"time":475.69}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"interrupt","system":{"cpu":{"time":0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"nice","system":{"cpu":{"time":0.1}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"softirq","system":{"cpu":{"time":0.57}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"steal","system":{"cpu":{"time":0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"system","system":{"cpu":{"time":15.88}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"user","system":{"cpu":{"time":50.09}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"name":"my-host"},"os":{"type":"linux"},"state":"wait","system":{"cpu":{"time":0.95}}}`

var expectedLogBodyWithEmptyTimestamp = `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`
var expectedLogBodyDeDottedWithEmptyTimestamp = `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Attributes":{"log-attr1":"value1"},"Body":"log-body","Resource":{"foo":{"bar":"baz"},"key1":"value1"},"Scope":{"name":"","version":""},"SeverityNumber":0,"TraceFlags":0}`

func TestEncodeSpan(t *testing.T) {
	model := &encodeModel{dedup: true, dedot: false}
	td := mockResourceSpans()
	spanByte, err := model.encodeSpan(td.ResourceSpans().At(0).Resource(), td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0), td.ResourceSpans().At(0).ScopeSpans().At(0).Scope())
	assert.NoError(t, err)
	assert.Equal(t, expectedSpanBody, string(spanByte))
}

func TestEncodeLog(t *testing.T) {
	t.Run("empty timestamp with observedTimestamp override", func(t *testing.T) {
		model := &encodeModel{dedup: true, dedot: false}
		td := mockResourceLogs()
		td.ScopeLogs().At(0).LogRecords().At(0).SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)))
		logByte, err := model.encodeLog(td.Resource(), td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope())
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBody, string(logByte))
	})

	t.Run("both timestamp and observedTimestamp empty", func(t *testing.T) {
		model := &encodeModel{dedup: true, dedot: false}
		td := mockResourceLogs()
		logByte, err := model.encodeLog(td.Resource(), td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope())
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBodyWithEmptyTimestamp, string(logByte))
	})

	t.Run("dedot true", func(t *testing.T) {
		model := &encodeModel{dedup: true, dedot: true}
		td := mockResourceLogs()
		td.Resource().Attributes().PutStr("foo.bar", "baz")
		logByte, err := model.encodeLog(td.Resource(), td.ScopeLogs().At(0).LogRecords().At(0), td.ScopeLogs().At(0).Scope())
		require.NoError(t, err)
		require.Equal(t, expectedLogBodyDeDottedWithEmptyTimestamp, string(logByte))
	})
}

func TestEncodeMetric(t *testing.T) {
	// Prepare metrics to test.
	metrics := createTestMetrics(t)

	// Encode the metrics.
	model := &encodeModel{
		dedot: true,
		dedup: true,
		mode:  MappingNone,
	}
	docsBytes, err := model.encodeMetrics(metrics.ResourceMetrics().At(0).Resource(), metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics(), metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope())

	// Convert the byte arrays to strings and sort the docs to make the test deterministic.
	require.NoError(t, err)
	docs := make([]string, 0, len(docsBytes))
	for _, docBytes := range docsBytes {
		docs = append(docs, string(docBytes))
	}
	sort.Strings(docs)
	allDocsSorted := strings.Join(docs, "\n")

	// Test that the result matches the expected value.
	assert.Equal(t, expectedMetricsEncoded, allDocsSorted)
}

func createTestMetrics(t *testing.T) pmetric.Metrics {
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	metricBytes, err := os.ReadFile("testdata/metrics-cpu.json")
	require.NoError(t, err)
	metrics, err := metricsUnmarshaler.UnmarshalMetrics(metricBytes)
	require.NoError(t, err)
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
	event.Attributes().PutStr("evnetMockFoo", "foo")
	event.Attributes().PutStr("evnetMockBar", "bar")
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

	attributes := pcommon.NewMap()
	err := attributes.FromRaw(map[string]any{
		"s": "baz",
		"o": map[string]any{
			"sub_i": 19,
		},
	})
	require.NoError(t, err)

	tests := map[string]struct {
		mappingMode MappingMode
		want        func() objmodel.Document
	}{
		"raw": {
			mappingMode: MappingRaw,
			want: func() objmodel.Document {
				return objmodel.DocumentFromAttributes(attributes)
			},
		},
		"none": {
			mappingMode: MappingNone,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddAttributes("Attributes", attributes)
				return doc
			},
		},
		"ecs": {
			mappingMode: MappingECS,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddAttributes("Attributes", attributes)
				return doc
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := encodeModel{
				mode: test.mappingMode,
			}

			doc := objmodel.Document{}
			m.encodeAttributes(&doc, attributes)
			require.Equal(t, test.want(), doc)
		})
	}
}

func TestEncodeEvents(t *testing.T) {
	t.Parallel()

	events := ptrace.NewSpanEventSlice()
	events.EnsureCapacity(4)
	for i := 0; i < 4; i++ {
		event := events.AppendEmpty()
		event.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Duration(i) * time.Minute)))
		event.SetName(fmt.Sprintf("event_%d", i))
	}

	tests := map[string]struct {
		mappingMode MappingMode
		want        func() objmodel.Document
	}{
		"raw": {
			mappingMode: MappingRaw,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("", events)
				return doc
			},
		},
		"none": {
			mappingMode: MappingNone,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("Events", events)
				return doc
			},
		},
		"ecs": {
			mappingMode: MappingECS,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("Events", events)
				return doc
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := encodeModel{
				mode: test.mappingMode,
			}

			doc := objmodel.Document{}
			m.encodeEvents(&doc, events)
			require.Equal(t, test.want(), doc)
		})
	}
}

func TestEncodeLogECSModeDuplication(t *testing.T) {
	resource := pcommon.NewResource()
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

	m := encodeModel{
		mode:  MappingECS,
		dedot: true,
		dedup: true,
	}
	doc, err := m.encodeLog(resource, record, scope)
	require.NoError(t, err)

	assert.Equal(t, want, string(doc))

}

func TestEncodeLogECSMode(t *testing.T) {
	resource := pcommon.NewResource()
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

	m := encodeModel{}
	doc := m.encodeLogECSMode(resource, record, scope)

	expectedDocFields := pcommon.NewMap()
	err = expectedDocFields.FromRaw(map[string]any{
		"service.name":               "foo.bar",
		"service.version":            "1.1.0",
		"service.node.name":          "i-103de39e0a",
		"agent.name":                 "opentelemetry/perl",
		"agent.version":              "7.9.12",
		"cloud.provider":             "gcp",
		"cloud.account.id":           "19347013",
		"cloud.region":               "us-west-1",
		"cloud.availability_zone":    "us-west-1b",
		"cloud.service.name":         "gke",
		"container.name":             "happy-seger",
		"container.id":               "e69cc5d3dda",
		"container.image.name":       "my-app",
		"container.runtime":          "docker",
		"host.hostname":              "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		"host.name":                  "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		"host.id":                    "i-103de39e0a",
		"host.type":                  "t2.medium",
		"host.architecture":          "x86_64",
		"process.pid":                9833,
		"process.command_line":       "/usr/bin/ssh -l user 10.0.0.16",
		"process.executable":         "/usr/bin/ssh",
		"service.runtime.name":       "OpenJDK Runtime Environment",
		"service.runtime.version":    "14.0.2",
		"host.os.platform":           "darwin",
		"host.os.full":               "Mac OS Mojave",
		"host.os.name":               "Mac OS X",
		"host.os.version":            "10.14.1",
		"host.os.type":               "macos",
		"device.id":                  "00000000-54b3-e7c7-0000-000046bffd97",
		"device.model.identifier":    "SM-G920F",
		"device.model.name":          "Samsung Galaxy S6",
		"device.manufacturer":        "Samsung",
		"event.action":               "user-password-change",
		"kubernetes.namespace":       "default",
		"kubernetes.node.name":       "node-1",
		"kubernetes.pod.name":        "opentelemetry-pod-autoconf",
		"kubernetes.pod.uid":         "275ecb36-5aa8-4c2a-9c47-d8bb681b9aff",
		"kubernetes.deployment.name": "coredns",
	})
	require.NoError(t, err)

	expectedDoc := objmodel.Document{}
	expectedDoc.AddAttributes("", expectedDocFields)
	expectedDoc.AddTimestamp("@timestamp", observedTimestamp)
	expectedDoc.Add("container.image.tag", objmodel.ArrValue(objmodel.StringValue("v3.4.0")))

	doc.Sort()
	expectedDoc.Sort()
	require.Equal(t, expectedDoc, doc)
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
			resource := pcommon.NewResource()
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

			m := encodeModel{}
			doc := m.encodeLogECSMode(resource, record, scope)

			expectedDoc := objmodel.Document{}
			expectedDoc.AddTimestamp("@timestamp", timestamp)
			expectedDoc.AddString("agent.name", test.expectedAgentName)

			doc.Sort()
			expectedDoc.Sort()
			require.Equal(t, expectedDoc, doc)
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
			resource := pcommon.NewResource()
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

			m := encodeModel{}
			doc := m.encodeLogECSMode(resource, record, scope)

			expectedDoc := objmodel.Document{}
			expectedDoc.AddTimestamp("@timestamp", timestamp)
			expectedDoc.AddString("agent.name", "otlp")
			expectedDoc.AddString("agent.version", test.expectedAgentVersion)

			doc.Sort()
			expectedDoc.Sort()
			require.Equal(t, expectedDoc, doc)
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
			resource := pcommon.NewResource()
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

			m := encodeModel{}
			doc := m.encodeLogECSMode(resource, record, scope)

			expectedDoc := objmodel.Document{}
			expectedDoc.AddTimestamp("@timestamp", timestamp)
			expectedDoc.AddString("agent.name", "otlp")
			if test.expectedHostOsName != "" {
				expectedDoc.AddString("host.os.name", test.expectedHostOsName)
			}
			if test.expectedHostOsType != "" {
				expectedDoc.AddString("host.os.type", test.expectedHostOsType)
			}
			if test.expectedHostOsPlatform != "" {
				expectedDoc.AddString("host.os.platform", test.expectedHostOsPlatform)
			}

			doc.Sort()
			expectedDoc.Sort()
			require.Equal(t, expectedDoc, doc)
		})
	}
}

func TestEncodeLogECSModeTimestamps(t *testing.T) {
	tests := map[string]struct {
		timeUnixNano         int64
		observedTimeUnixNano int64
		expectedTimestamp    time.Time
	}{
		"only_observed_set": {
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    time.Unix(0, 1710273641123456789),
		},
		"both_set": {
			timeUnixNano:         1710273639345678901,
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    time.Unix(0, 1710273639345678901),
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

			m := encodeModel{}
			doc := m.encodeLogECSMode(resource, record, scope)

			expectedDoc := objmodel.Document{}
			expectedDoc.AddTimestamp("@timestamp", pcommon.NewTimestampFromTime(test.expectedTimestamp))
			expectedDoc.AddString("agent.name", "otlp")

			doc.Sort()
			expectedDoc.Sort()
			require.Equal(t, expectedDoc, doc)
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
			encodeLogAttributesECSMode(&doc, test.attrs(), test.conversionMap, test.preserveMap)

			doc.Sort()
			expectedDoc := test.expectedDoc()
			expectedDoc.Sort()
			require.Equal(t, expectedDoc, doc)
		})
	}
}
