// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

var expectedSpanBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.service.instance.id":"23","Duration":1000000,"EndTimestamp":"2023-04-19T03:04:06.000000006Z","Events.fooEvent.evnetMockBar":"bar","Events.fooEvent.evnetMockFoo":"foo","Events.fooEvent.time":"2023-04-19T03:04:05.000000006Z","Kind":"SPAN_KIND_CLIENT","Link":"[{\"attribute\":{},\"spanID\":\"\",\"traceID\":\"01020304050607080807060504030200\"}]","Name":"client span","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","Scope.lib-foo":"lib-bar","Scope.name":"io.opentelemetry.rabbitmq-2.7","Scope.version":"1.30.0-alpha","SpanId":"1920212223242526","TraceId":"01020304050607080807060504030201","TraceStatus":2,"TraceStatusDescription":"Test"}`

var expectedLogBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`

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

func TestEncodeLogECSMode(t *testing.T) {
	resource := pcommon.NewResource()
	err := resource.Attributes().FromRaw(map[string]any{
		semconv.AttributeServiceName:           "foo.bar",
		semconv.AttributeServiceVersion:        "1.1.0",
		semconv.AttributeServiceInstanceID:     "i-103de39e0a",
		semconv.AttributeTelemetrySDKName:      "perl-otel",
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
		semconv.AttributeOSType:                "darwin",
		semconv.AttributeOSDescription:         "Mac OS Mojave",
		semconv.AttributeOSName:                "Mac OS X",
		semconv.AttributeOSVersion:             "10.14.1",
		semconv.AttributeDeviceID:              "00000000-54b3-e7c7-0000-000046bffd97",
		semconv.AttributeDeviceModelIdentifier: "SM-G920F",
		semconv.AttributeDeviceModelName:       "Samsung Galaxy S6",
		semconv.AttributeDeviceManufacturer:    "Samsung",
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
	now := time.Now()
	doc := m.encodeLogECSMode(resource, record, scope, now)

	expectedDocFields := pcommon.NewMap()
	err = expectedDocFields.FromRaw(map[string]any{
		"service.name":            "foo.bar",
		"service.version":         "1.1.0",
		"service.node.name":       "i-103de39e0a",
		"agent.name":              "perl-otel",
		"agent.version":           "7.9.12",
		"service.language.name":   "perl",
		"cloud.provider":          "gcp",
		"cloud.account.id":        "19347013",
		"cloud.region":            "us-west-1",
		"cloud.availability_zone": "us-west-1b",
		"cloud.service.name":      "gke",
		"container.name":          "happy-seger",
		"container.id":            "e69cc5d3dda",
		"container.image.name":    "my-app",
		"container.runtime":       "docker",
		"host.hostname":           "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		"host.id":                 "i-103de39e0a",
		"host.type":               "t2.medium",
		"host.architecture":       "x86_64",
		"process.pid":             9833,
		"process.command_line":    "/usr/bin/ssh -l user 10.0.0.16",
		"process.executable":      "/usr/bin/ssh",
		"os.platform":             "darwin",
		"os.full":                 "Mac OS Mojave",
		"os.name":                 "Mac OS X",
		"os.version":              "10.14.1",
		"device.id":               "00000000-54b3-e7c7-0000-000046bffd97",
		"device.model.identifier": "SM-G920F",
		"device.model.name":       "Samsung Galaxy S6",
		"device.manufacturer":     "Samsung",
		"event.action":            "user-password-change",
	})
	require.NoError(t, err)

	expectedDoc := objmodel.Document{}
	expectedDoc.AddAttributes("", expectedDocFields)
	expectedDoc.AddTimestamp("@timestamp", observedTimestamp)
	expectedDoc.Add("event.received", objmodel.TimestampValue(now))
	expectedDoc.Add("container.image.tag", objmodel.ArrValue(objmodel.StringValue("v3.4.0")))

	doc.Sort()
	expectedDoc.Sort()
	require.Equal(t, expectedDoc, doc)
}

func TestEncodeLogECSModeRecordTimestamps(t *testing.T) {
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
			now := time.Now()
			doc := m.encodeLogECSMode(resource, record, scope, now)

			expectedDoc := objmodel.Document{}
			expectedDoc.AddTimestamp("@timestamp", pcommon.NewTimestampFromTime(test.expectedTimestamp))
			expectedDoc.Add("event.received", objmodel.TimestampValue(now))

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
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var doc objmodel.Document
			encodeLogAttributesECSMode(&doc, test.attrs(), test.conversionMap)

			doc.Sort()
			expectedDoc := test.expectedDoc()
			expectedDoc.Sort()
			require.Equal(t, expectedDoc, doc)
		})
	}
}
