// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chjson

import (
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/column/orderedmap"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func testMap() pcommon.Map {
	attrs := pcommon.NewMap()

	attrs.PutStr("app", "telemetry-service")
	attrs.PutInt("process.pid", 12345)
	attrs.PutBool("debug.enabled", true)
	attrs.PutEmpty("request_id")

	serverInfo := attrs.PutEmptyMap("server.info")
	serverInfo.PutStr("hostname", "prod-server-01")
	serverInfo.PutDouble("uptime", 156.7)
	serverMetadata := serverInfo.PutEmptyMap("metadata")
	serverMetadata.PutStr("region", "us-west-2")
	serverMetadata.PutStr("tier", "premium")

	userRoles := attrs.PutEmptySlice("user.roles")
	userRoles.AppendEmpty().SetStr("admin")
	userRoles.AppendEmpty().SetStr("editor")
	userRoles.AppendEmpty().SetStr("viewer")

	systemMetrics := attrs.PutEmptySlice("system.metrics")
	systemMetrics.AppendEmpty().SetDouble(0.75)
	systemMetrics.AppendEmpty().SetDouble(0.83)
	systemMetrics.AppendEmpty().SetDouble(0.3335)

	resourceUsage := attrs.PutEmptyMap("resource.usage")
	cpu := resourceUsage.PutEmptyMap("cpu")
	cpu.PutInt("cores", 8)
	cpuLoads := cpu.PutEmptySlice("loads")
	cpuLoads.AppendEmpty().SetDouble(1.5)
	cpuLoads.AppendEmpty().SetDouble(2.0)
	cpuLoads.AppendEmpty().SetDouble(1.8)

	memory := resourceUsage.PutEmptyMap("memory")
	memory.PutInt("total", 16384)
	memory.PutInt("used", 8192)
	memAllocs := memory.PutEmptySlice("allocations")
	memAllocs.AppendEmpty().SetInt(1024)
	memAllocs.AppendEmpty().SetInt(2048)
	memAllocs.AppendEmpty().SetInt(4096)

	secretBytes := attrs.PutEmptyBytes("secret")
	for i := 0; i < 10; i++ {
		secretBytes.Append(0xA, 0xB, 0xC, 0xD)
	}

	return attrs
}

func TestAttributesToJSON(t *testing.T) {
	m := testMap()

	jb := JSONBuffer{
		buf:          make([]byte, 0, 1024),
		base64Buffer: make([]byte, 0, 128),
	}

	AttributesToJSON(&jb, m)
	// Verify resetting correctly encodes the right output, including base64 buffer
	jb.Reset()
	AttributesToJSON(&jb, m)

	actual := string(jb.Bytes())
	expected := `{"app":"telemetry-service","process.pid":12345,"debug.enabled":true,"request_id":null,` +
		`"server.info":{"hostname":"prod-server-01","uptime":156.7,"metadata":{"region":"us-west-2",` +
		`"tier":"premium"}},"user.roles":["admin","editor","viewer"],"system.metrics":[0.75,0.83,0.3335],` +
		`"resource.usage":{"cpu":{"cores":8,"loads":[1.5,2,1.8]},"memory":{"total":16384,"used":8192,` +
		`"allocations":[1024,2048,4096]}},"secret":"CgsMDQoLDA0KCwwNCgsMDQoLDA0KCwwNCgsMDQoLDA0KCwwNCgsMDQ=="}`

	require.Equal(t, expected, actual)
}

func TestAppendSpanIDToHex(t *testing.T) {
	hexBuf := make([]byte, 0, 128)

	spanID := pcommon.SpanID{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
	}

	var actual string
	for i := 0; i < 10; i++ {
		hexBuf = AppendSpanIDToHex(hexBuf[:0], spanID)
		actual = string(hexBuf)
	}

	require.Equal(t, "0011223344556677", actual)
}

func TestAppendTraceIDToHex(t *testing.T) {
	hexBuf := make([]byte, 0, 128)

	traceID := pcommon.TraceID{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF,
	}

	var actual string
	for i := 0; i < 10; i++ {
		hexBuf = AppendTraceIDToHex(hexBuf[:0], traceID)
		actual = string(hexBuf)
	}

	require.Equal(t, "00112233445566778899aabbccddeeff", actual)
}

func BenchmarkAppendSpanIDToHex(b *testing.B) {
	hexBuf := make([]byte, 0, 128)

	spanID := pcommon.SpanID{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hexBuf = AppendSpanIDToHex(hexBuf[:0], spanID)
	}
}

func BenchmarkAppendTraceIDToHex(b *testing.B) {
	hexBuf := make([]byte, 0, 128)

	traceID := pcommon.TraceID{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hexBuf = AppendTraceIDToHex(hexBuf[:0], traceID)
	}
}

func BenchmarkAttributesToJSON(b *testing.B) {
	m := testMap()
	jb := JSONBuffer{
		buf:          make([]byte, 0, 1024),
		base64Buffer: make([]byte, 0, 128),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jb.Reset()
		AttributesToJSON(&jb, m)
	}
}

func attributesToMap(attributes pcommon.Map) column.IterableOrderedMap {
	return orderedmap.CollectN(func(yield func(string, string) bool) {
		attributes.Range(func(k string, v pcommon.Value) bool {
			return yield(k, v.AsString())
		})
	}, attributes.Len())
}

func BenchmarkAttributesToMap(b *testing.B) {
	m := testMap()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attributesToMap(m)
	}
}
