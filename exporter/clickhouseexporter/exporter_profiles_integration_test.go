// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap/zaptest"
)

func testProfilesExporter(t *testing.T, endpoint string) {
	exporter := newTestProfilesExporter(t, endpoint)
	verifyExportProfiles(t, exporter)
}

func newTestProfilesExporter(t *testing.T, dsn string, fns ...func(*Config)) *profilesExporter {
	exporter := newProfilesExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(t.Context(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })
	return exporter
}

func verifyExportProfiles(t *testing.T, exporter *profilesExporter) {
	pushConcurrentlyNoError(t, func() error {
		return exporter.pushProfilesData(t.Context(), simpleProfiles(5000))
	})

	type profile struct {
		Timestamp          time.Time         `ch:"Timestamp"`
		ProfileID          string            `ch:"ProfileId"`
		SampleType         string            `ch:"SampleType"`
		SampleUnit         string            `ch:"SampleUnit"`
		ServiceName        string            `ch:"ServiceName"`
		ResourceAttributes map[string]string `ch:"ResourceAttributes"`
		ScopeName          string            `ch:"ScopeName"`
		ScopeVersion       string            `ch:"ScopeVersion"`
		ProfileAttributes  map[string]string `ch:"ProfileAttributes"`
		SampleAttributes   map[string]string `ch:"SampleAttributes"`
		StackHash          uint64            `ch:"StackHash"`
		Addresses          []uint64          `ch:"Addresses"`
		FunctionNames      []string          `ch:"FunctionNames"`
		FileNames          []string          `ch:"FileNames"`
		LineNumbers        []int32           `ch:"LineNumbers"`
		MappingFileNames   []string          `ch:"MappingFileNames"`
		Values             []int64           `ch:"Values"`
		TimestampsUnixNano []uint64          `ch:"TimestampsUnixNano"`
		DurationNano       uint64            `ch:"DurationNano"`
		Period             int64             `ch:"Period"`
		PeriodType         string            `ch:"PeriodType"`
		PeriodUnit         string            `ch:"PeriodUnit"`
		TraceID            string            `ch:"TraceId"`
		SpanID             string            `ch:"SpanId"`
	}

	scratch := pprofile.NewProfiles()
	buildTestDictionary(scratch.Dictionary())
	expectedHash := resolveStack(scratch.Dictionary(), 1).hash

	expectedProfile := profile{
		Timestamp:          telemetryTimestamp,
		ProfileID:          "42000000000000000000000000000000",
		SampleType:         "cpu",
		SampleUnit:         "nanoseconds",
		ServiceName:        "test-service",
		ResourceAttributes: map[string]string{"service.name": "test-service"},
		ScopeName:          "io.opentelemetry.contrib.clickhouse",
		ScopeVersion:       "1.0.0",
		ProfileAttributes:  map[string]string{"profile.tag": "prod"},
		SampleAttributes:   map[string]string{"thread.name": "worker-1"},
		StackHash:          expectedHash,
		Addresses:          []uint64{0x1000, 0x2000, 0x2000},
		FunctionNames:      []string{"foo", "bar", "main"},
		FileNames:          []string{"foo.go", "bar.go", "main.go"},
		LineNumbers:        []int32{42, 10, 99},
		MappingFileNames:   []string{"libc.so", "libc.so", "libc.so"},
		Values:             []int64{1500000},
		TimestampsUnixNano: []uint64{uint64(telemetryTimestamp.UnixNano())},
		DurationNano:       60000000000,
		Period:             10000000,
		PeriodType:         "cpu",
		PeriodUnit:         "nanoseconds",
		TraceID:            "0102030405060708090a0b0c0d0e0f10",
		SpanID:             "0102030405060708",
	}

	row := exporter.db.QueryRow(t.Context(), "SELECT * FROM otel_int_test.otel_profiles LIMIT 1")
	require.NoError(t, row.Err())

	var actualProfile profile
	require.NoError(t, row.ScanStruct(&actualProfile))
	require.Equal(t, expectedProfile, actualProfile)
}

func simpleProfiles(count int) pprofile.Profiles {
	profiles := pprofile.NewProfiles()
	buildTestDictionary(profiles.Dictionary())

	rp := profiles.ResourceProfiles().AppendEmpty()
	rp.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
	rp.Resource().Attributes().PutStr("service.name", "test-service")

	sp := rp.ScopeProfiles().AppendEmpty()
	sp.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
	sp.Scope().SetName("io.opentelemetry.contrib.clickhouse")
	sp.Scope().SetVersion("1.0.0")

	profile := sp.Profiles().AppendEmpty()
	profile.SetProfileID(pprofile.ProfileID{0x42})
	profile.SetTime(pcommon.NewTimestampFromTime(telemetryTimestamp))
	profile.SetDurationNano(60000000000)
	profile.SetPeriod(10000000)
	profile.SampleType().SetTypeStrindex(1) // cpu
	profile.SampleType().SetUnitStrindex(2) // nanoseconds
	profile.PeriodType().SetTypeStrindex(1) // cpu
	profile.PeriodType().SetUnitStrindex(2) // nanoseconds
	profile.AttributeIndices().Append(2)    // profile.tag=prod

	for range count {
		sample := profile.Samples().AppendEmpty()
		sample.SetStackIndex(1)
		sample.SetLinkIndex(1)
		sample.AttributeIndices().Append(1) // thread.name=worker-1
		sample.Values().Append(1500000)
		sample.TimestampsUnixNano().Append(uint64(telemetryTimestamp.UnixNano()))
	}

	return profiles
}
