// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCompliantPrometheusUnit(t *testing.T) {
	require.Equal(t, "bytes", BuildCompliantPrometheusUnit("By"))
	require.Equal(t, "microseconds", BuildCompliantPrometheusUnit("us"))
	require.Equal(t, "connections", BuildCompliantPrometheusUnit("connections"))
	require.Equal(t, "gibibytes_per_hour", BuildCompliantPrometheusUnit("GiBy/h"))
	require.Empty(t, BuildCompliantPrometheusUnit("{objects}"))
	require.Empty(t, BuildCompliantPrometheusUnit("{scanned}/{returned}"))
	require.Equal(t, "per_second", BuildCompliantPrometheusUnit("{objects}/s"))
	require.Equal(t, "percent", BuildCompliantPrometheusUnit("%"))
	require.Empty(t, BuildCompliantPrometheusUnit("1"))
}

func TestBuildCompliantMainUnit(t *testing.T) {
	require.Equal(t, "bytes", buildCompliantMainUnit("By"))
	require.Equal(t, "microseconds", buildCompliantMainUnit("us"))
	require.Equal(t, "connections", buildCompliantMainUnit("connections"))
	require.Equal(t, "gibibytes", buildCompliantMainUnit("GiBy/h"))
	require.Empty(t, buildCompliantMainUnit("{objects}"))
	require.Empty(t, buildCompliantMainUnit("{scanned}/{returned}"))
	require.Empty(t, buildCompliantMainUnit("{objects}/s"))
	require.Equal(t, "percent", buildCompliantMainUnit("%"))
	require.Empty(t, buildCompliantMainUnit("1"))
}

func TestBuildCompliantPerUnit(t *testing.T) {
	require.Empty(t, buildCompliantPerUnit("By"))
	require.Empty(t, buildCompliantPerUnit("us"))
	require.Empty(t, buildCompliantPerUnit("connections"))
	require.Equal(t, "hour", buildCompliantPerUnit("GiBy/h"))
	require.Empty(t, buildCompliantPerUnit("{objects}"))
	require.Empty(t, buildCompliantPerUnit("{scanned}/{returned}"))
	require.Equal(t, "second", buildCompliantPerUnit("{objects}/s"))
	require.Empty(t, buildCompliantPerUnit("%"))
	require.Empty(t, buildCompliantPerUnit("1"))
}

func TestUnitMapGetOrDefault(t *testing.T) {
	require.Empty(t, unitMapGetOrDefault(""))
	require.Equal(t, "seconds", unitMapGetOrDefault("s"))
	require.Equal(t, "invalid", unitMapGetOrDefault("invalid"))
}

func TestPerUnitMapGetOrDefault(t *testing.T) {
	require.Empty(t, perUnitMapGetOrDefault(""))
	require.Equal(t, "second", perUnitMapGetOrDefault("s"))
	require.Equal(t, "invalid", perUnitMapGetOrDefault("invalid"))
}

func TestCleanUpString(t *testing.T) {
	require.Empty(t, CleanUpString(""))
	require.Equal(t, "a_b", CleanUpString("a b"))
	require.Equal(t, "hello_world", CleanUpString("hello, world!"))
	require.Equal(t, "hello_you_2", CleanUpString("hello you 2"))
	require.Equal(t, "1000", CleanUpString("$1000"))
	require.Empty(t, CleanUpString("*+$^=)"))
}
