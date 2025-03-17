// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCompliantPrometheusUnit(t *testing.T) {
	require.Equal(t, "bytes", BuildCompliantPrometheusUnit(createGauge("system.filesystem.usage", "By").Unit()))
	require.Equal(t, "microseconds", BuildCompliantPrometheusUnit(createGauge("redis.latest_fork", "us").Unit()))
	require.Equal(t, "connections", BuildCompliantPrometheusUnit(createGauge("apache.current_connections", "connections").Unit()))
	require.Equal(t, "gibibytes_per_hour", BuildCompliantPrometheusUnit(createGauge("mongodbatlas.process.oplog.rate", "GiBy/h").Unit()))
	require.Equal(t, "", BuildCompliantPrometheusUnit(createCounter("active_directory.ds.replication.sync.object.pending", "{objects}").Unit()))
	require.Equal(t, "", BuildCompliantPrometheusUnit(createGauge("mongodbatlas.process.db.query_targeting.scanned_per_returned", "{scanned}/{returned}").Unit()))
	require.Equal(t, "per_second", BuildCompliantPrometheusUnit(createGauge("active_directory.ds.replication.object.rate", "{objects}/s").Unit()))
	require.Equal(t, "percent", BuildCompliantPrometheusUnit(createGauge("active_directory.ds.name_cache.hit_rate", "%").Unit()))
	require.Equal(t, "", BuildCompliantPrometheusUnit(createGauge("system.cpu.utilization", "1").Unit()))
}

func TestBuildCompliantMainUnit(t *testing.T) {
	require.Equal(t, "bytes", BuildCompliantMainUnit(createGauge("system.filesystem.usage", "By").Unit()))
	require.Equal(t, "microseconds", BuildCompliantMainUnit(createGauge("redis.latest_fork", "us").Unit()))
	require.Equal(t, "connections", BuildCompliantMainUnit(createGauge("apache.current_connections", "connections").Unit()))
	require.Equal(t, "gibibytes", BuildCompliantMainUnit(createGauge("mongodbatlas.process.oplog.rate", "GiBy/h").Unit()))
	require.Equal(t, "", BuildCompliantMainUnit(createCounter("active_directory.ds.replication.sync.object.pending", "{objects}").Unit()))
	require.Equal(t, "", BuildCompliantMainUnit(createGauge("mongodbatlas.process.db.query_targeting.scanned_per_returned", "{scanned}/{returned}").Unit()))
	require.Equal(t, "", BuildCompliantMainUnit(createGauge("active_directory.ds.replication.object.rate", "{objects}/s").Unit()))
	require.Equal(t, "percent", BuildCompliantMainUnit(createGauge("active_directory.ds.name_cache.hit_rate", "%").Unit()))
	require.Equal(t, "", BuildCompliantMainUnit(createGauge("system.cpu.utilization", "1").Unit()))
}

func TestBuildCompliantPerUnit(t *testing.T) {
	require.Equal(t, "", BuildCompliantPerUnit(createGauge("system.filesystem.usage", "By").Unit()))
	require.Equal(t, "", BuildCompliantPerUnit(createGauge("redis.latest_fork", "us").Unit()))
	require.Equal(t, "", BuildCompliantPerUnit(createGauge("apache.current_connections", "connections").Unit()))
	require.Equal(t, "hour", BuildCompliantPerUnit(createGauge("mongodbatlas.process.oplog.rate", "GiBy/h").Unit()))
	require.Equal(t, "", BuildCompliantPerUnit(createCounter("active_directory.ds.replication.sync.object.pending", "{objects}").Unit()))
	require.Equal(t, "", BuildCompliantPerUnit(createGauge("mongodbatlas.process.db.query_targeting.scanned_per_returned", "{scanned}/{returned}").Unit()))
	require.Equal(t, "second", BuildCompliantPerUnit(createGauge("active_directory.ds.replication.object.rate", "{objects}/s").Unit()))
	require.Equal(t, "", BuildCompliantPerUnit(createGauge("active_directory.ds.name_cache.hit_rate", "%").Unit()))
	require.Equal(t, "", BuildCompliantPerUnit(createGauge("system.cpu.utilization", "1").Unit()))
}

func TestUnitMapGetOrDefault(t *testing.T) {
	require.Equal(t, "", unitMapGetOrDefault(""))
	require.Equal(t, "seconds", unitMapGetOrDefault("s"))
	require.Equal(t, "invalid", unitMapGetOrDefault("invalid"))
}

func TestPerUnitMapGetOrDefault(t *testing.T) {
	require.Equal(t, "", perUnitMapGetOrDefault(""))
	require.Equal(t, "second", perUnitMapGetOrDefault("s"))
	require.Equal(t, "invalid", perUnitMapGetOrDefault("invalid"))
}

func TestCleanUpString(t *testing.T) {
	require.Equal(t, "", CleanUpString(""))
	require.Equal(t, "a_b", CleanUpString("a b"))
	require.Equal(t, "hello_world", CleanUpString("hello, world!"))
	require.Equal(t, "hello_you_2", CleanUpString("hello you 2"))
	require.Equal(t, "1000", CleanUpString("$1000"))
	require.Equal(t, "", CleanUpString("*+$^=)"))
}
