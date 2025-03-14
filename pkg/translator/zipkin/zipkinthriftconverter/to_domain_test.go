// Copyright The OpenTelemetry Authors
// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
// SPDX-License-Identifier: Apache-2.0

package zipkin

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/jaegertracing/jaeger-idl/model/v1"
	z "github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const NumberOfFixtures = 3

func TestToDomain(t *testing.T) {
	for i := 1; i <= NumberOfFixtures; i++ {
		in := fmt.Sprintf("fixtures/zipkin_%02d.json", i)
		out := fmt.Sprintf("fixtures/domain_%02d.json", i)
		zSpans := loadZipkinSpans(t, in)
		expectedTrace := loadJaegerTrace(t, out)
		expectedTrace.NormalizeTimestamps()
		name := in + " -> " + out + " : " + zSpans[0].Name
		t.Run(name, func(t *testing.T) {
			trace, err := ToDomain(zSpans)
			require.NoError(t, err)
			trace.NormalizeTimestamps()
			if !assert.Equal(t, expectedTrace, trace) {
				for _, err := range pretty.Diff(expectedTrace, trace) {
					t.Log(err)
				}
				out, err := json.Marshal(trace)
				require.NoError(t, err)
				t.Logf("Actual trace: %s", string(out))
			}
		})
		if i == 1 {
			t.Run("ToDomainSpans", func(t *testing.T) {
				zSpan := zSpans[0]
				jSpans, err := ToDomainSpan(zSpan)
				require.NoError(t, err)
				for _, jSpan := range jSpans {
					jSpan.NormalizeTimestamps()
					assert.Equal(t, expectedTrace.Spans[0], jSpan)
				}
			})
		}
	}
}

func TestToDomainNoServiceNameError(t *testing.T) {
	zSpans := getZipkinSpans(t, `[{ "trace_id": -1, "id": 31 }]`)
	trc, err := ToDomain(zSpans)
	require.EqualError(t, err, "cannot find service name in Zipkin span [traceID=ffffffffffffffff, spanID=1f]")
	assert.Len(t, trc.Spans, 1)
	assert.Equal(t, "unknown-service-name", trc.Spans[0].Process.ServiceName)
}

func TestToDomainServiceNameInBinAnnotation(t *testing.T) {
	zSpans := getZipkinSpans(t, `[{ "trace_id": -1, "id": 31,
	"binary_annotations": [{"key": "foo", "host": {"service_name": "bar", "ipv4": 23456}}] }]`)
	trc, err := ToDomain(zSpans)
	require.NoError(t, err)
	assert.Len(t, trc.Spans, 1)
	assert.Equal(t, "bar", trc.Spans[0].Process.ServiceName)
}

func TestToDomainWithDurationFromServerAnnotations(t *testing.T) {
	zSpans := getZipkinSpans(t, `[{ "trace_id": -1, "id": 31, "annotations": [
	{"value": "sr", "timestamp": 1, "host": {"service_name": "bar", "ipv4": 23456}},
	{"value": "ss", "timestamp": 10, "host": {"service_name": "bar", "ipv4": 23456}}
	]}]`)
	trc, err := ToDomain(zSpans)
	require.NoError(t, err)
	assert.Equal(t, 1000, trc.Spans[0].StartTime.Nanosecond())
	assert.Equal(t, 9000, int(trc.Spans[0].Duration))
}

func TestToDomainWithDurationFromClientAnnotations(t *testing.T) {
	zSpans := getZipkinSpans(t, `[{ "trace_id": -1, "id": 31, "annotations": [
	{"value": "cs", "timestamp": 1, "host": {"service_name": "bar", "ipv4": 23456}},
	{"value": "cr", "timestamp": 10, "host": {"service_name": "bar", "ipv4": 23456}}
	]}]`)
	trc, err := ToDomain(zSpans)
	require.NoError(t, err)
	assert.Equal(t, 1000, trc.Spans[0].StartTime.Nanosecond())
	assert.Equal(t, 9000, int(trc.Spans[0].Duration))
}

func TestToDomainMultipleSpanKinds(t *testing.T) {
	tests := []struct {
		json         string
		tagFirstKey  string
		tagSecondKey string
		tagFirstVal  model.SpanKind
		tagSecondVal model.SpanKind
	}{
		{
			json: `[{ "trace_id": -1, "id": 31, "annotations": [
		{"value": "cs", "host": {"service_name": "bar", "ipv4": 23456}},
		{"value": "sr", "timestamp": 1, "host": {"service_name": "bar", "ipv4": 23456}},
		{"value": "ss", "timestamp": 2, "host": {"service_name": "bar", "ipv4": 23456}}
		]}]`,
			tagFirstKey:  model.SpanKindKey,
			tagSecondKey: model.SpanKindKey,
			tagFirstVal:  model.SpanKindClient,
			tagSecondVal: model.SpanKindServer,
		},
		{
			json: `[{ "trace_id": -1, "id": 31, "annotations": [
		{"value": "sr", "host": {"service_name": "bar", "ipv4": 23456}},
		{"value": "cs", "timestamp": 1, "host": {"service_name": "bar", "ipv4": 23456}},
		{"value": "cr", "timestamp": 2, "host": {"service_name": "bar", "ipv4": 23456}}
		]}]`,
			tagFirstKey:  model.SpanKindKey,
			tagSecondKey: model.SpanKindKey,
			tagFirstVal:  model.SpanKindServer,
			tagSecondVal: model.SpanKindClient,
		},
	}

	for _, test := range tests {
		trc, err := ToDomain(getZipkinSpans(t, test.json))
		require.NoError(t, err)

		assert.Len(t, trc.Spans, 2)
		assert.Len(t, trc.Spans[0].Tags, 1)
		assert.Equal(t, test.tagFirstKey, trc.Spans[0].Tags[0].Key)
		assert.EqualValues(t, test.tagFirstVal, trc.Spans[0].Tags[0].VStr)

		assert.Len(t, trc.Spans[1].Tags, 1)
		assert.Equal(t, test.tagSecondKey, trc.Spans[1].Tags[0].Key)
		assert.Equal(t, time.Duration(1000), trc.Spans[1].Duration)
		assert.EqualValues(t, test.tagSecondVal, trc.Spans[1].Tags[0].VStr)
	}
}

func TestInvalidAnnotationTypeError(t *testing.T) {
	_, err := toDomain{}.transformBinaryAnnotation(&z.BinaryAnnotation{
		AnnotationType: -1,
	})
	require.EqualError(t, err, "unknown zipkin annotation type: -1")
}

// TestZipkinEncoding is just for reference to explain the base64 strings
// used in zipkin_03.json and jaeger_03.json fixtures
func TestValidateBase64Values(t *testing.T) {
	numberToBase64 := func(num any) string {
		buf := &bytes.Buffer{}
		err := binary.Write(buf, binary.BigEndian, num)
		require.NoError(t, err)
		encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
		return encoded
	}

	assert.Equal(t, "MDk=", numberToBase64(int16(12345)))
	assert.Equal(t, "AAAwOQ==", numberToBase64(int32(12345)))
	assert.Equal(t, "AAAAAAAAMDk=", numberToBase64(int64(12345)))
	assert.Equal(t, "QMgcgAAAAAA=", numberToBase64(int64(math.Float64bits(12345))))
	assert.Equal(t, int64(4668012349850910720), int64(math.Float64bits(12345)), "sanity check")
}

func loadZipkinSpans(t *testing.T, file string) []*z.Span {
	var zSpans []*z.Span
	loadJSON(t, file, &zSpans)
	return zSpans
}

func loadJaegerTrace(t *testing.T, file string) *model.Trace {
	var trc model.Trace
	loadJSONPB(t, file, &trc)
	return &trc
}

func loadJSONPB(t *testing.T, fileName string, obj proto.Message) {
	jsonFile, err := os.Open(fileName)
	require.NoError(t, err, "Failed to open json fixture file %s", fileName)
	require.NoError(t, jsonpb.Unmarshal(jsonFile, obj), fileName)
}

func getZipkinSpans(t *testing.T, s string) []*z.Span {
	var zSpans []*z.Span
	require.NoError(t, json.Unmarshal([]byte(s), &zSpans))
	return zSpans
}

func loadJSON(t *testing.T, fileName string, i any) {
	jsonFile, err := os.Open(fileName)
	require.NoError(t, err, "Failed to load json fixture file %s", fileName)
	jsonParser := json.NewDecoder(jsonFile)
	err = jsonParser.Decode(i)
	require.NoError(t, err, "Failed to parse json fixture file %s", fileName)
}
