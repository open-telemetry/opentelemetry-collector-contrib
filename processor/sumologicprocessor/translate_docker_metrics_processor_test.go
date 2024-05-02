// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestTranslateDockerMetric_NamesAreTranslatedCorrectly(t *testing.T) {
	testcases := []struct {
		nameIn  string
		nameOut string
	}{
		{nameIn: "container.cpu.percent", nameOut: "cpu_percentage"},
		{nameIn: "container.cpu.usage.system", nameOut: "system_cpu_usage"},
		{nameIn: "container.cpu.usage.percpu", nameOut: "cpu_usage.percpu_usage"},
		{nameIn: "container.cpu.usage.total", nameOut: "cpu_usage.total_usage"},
		{nameIn: "container.cpu.usage.kernelmode", nameOut: "cpu_usage.usage_in_kernelmode"},
		{nameIn: "container.cpu.usage.usermode", nameOut: "cpu_usage.usage_in_usermode"},
		{nameIn: "container.cpu.throttling_data.periods", nameOut: "throttling_data.periods"},
		{nameIn: "container.cpu.throttling_data.throttled_periods", nameOut: "throttling_data.throttled_periods"},
		{nameIn: "container.cpu.throttling_data.throttled_time", nameOut: "throttling_data.throttled_time"},
		{nameIn: "container.memory.usage.limit", nameOut: "limit"},
		{nameIn: "container.memory.usage.max", nameOut: "max_usage"},
		{nameIn: "container.memory.percent", nameOut: "memory_percentage"},
		{nameIn: "container.memory.usage.total", nameOut: "usage"},
		{nameIn: "container.memory.active_anon", nameOut: "stats.active_anon"},
		{nameIn: "container.memory.active_file", nameOut: "stats.active_file"},
		{nameIn: "container.memory.cache", nameOut: "stats.cache"},
		{nameIn: "container.memory.hierarchical_memory_limit", nameOut: "stats.hierarchical_memory_limit"},
		{nameIn: "container.memory.inactive_anon", nameOut: "stats.inactive_anon"},
		{nameIn: "container.memory.inactive_file", nameOut: "stats.inactive_file"},
		{nameIn: "container.memory.mapped_file", nameOut: "stats.mapped_file"},
		{nameIn: "container.memory.pgfault", nameOut: "stats.pgfault"},
		{nameIn: "container.memory.pgmajfault", nameOut: "stats.pgmajfault"},
		{nameIn: "container.memory.pgpgin", nameOut: "stats.pgpgin"},
		{nameIn: "container.memory.pgpgout", nameOut: "stats.pgpgout"},
		{nameIn: "container.memory.rss", nameOut: "stats.rss"},
		{nameIn: "container.memory.rss_huge", nameOut: "stats.rss_huge"},
		{nameIn: "container.memory.unevictable", nameOut: "stats.unevictable"},
		{nameIn: "container.memory.writeback", nameOut: "stats.writeback"},
		{nameIn: "container.memory.total_active_anon", nameOut: "stats.total_active_anon"},
		{nameIn: "container.memory.total_active_file", nameOut: "stats.total_active_file"},
		{nameIn: "container.memory.total_cache", nameOut: "stats.total_cache"},
		{nameIn: "container.memory.total_inactive_anon", nameOut: "stats.total_inactive_anon"},
		{nameIn: "container.memory.total_mapped_file", nameOut: "stats.total_mapped_file"},
		{nameIn: "container.memory.total_pgfault", nameOut: "stats.total_pgfault"},
		{nameIn: "container.memory.total_pgmajfault", nameOut: "stats.total_pgmajfault"},
		{nameIn: "container.memory.total_pgpgin", nameOut: "stats.total_pgpgin"},
		{nameIn: "container.memory.total_pgpgout", nameOut: "stats.total_pgpgout"},
		{nameIn: "container.memory.total_rss", nameOut: "stats.total_rss"},
		{nameIn: "container.memory.total_rss_huge", nameOut: "stats.total_rss_huge"},
		{nameIn: "container.memory.total_unevictable", nameOut: "stats.total_unevictable"},
		{nameIn: "container.memory.total_writeback", nameOut: "stats.total_writeback"},
		{nameIn: "container.blockio.io_merged_recursive", nameOut: "io_merged_recursive"},
		{nameIn: "container.blockio.io_queued_recursive", nameOut: "io_queue_recursive"},
		{nameIn: "container.blockio.io_service_bytes_recursive", nameOut: "io_service_bytes_recursive"},
		{nameIn: "container.blockio.io_service_time_recursive", nameOut: "io_service_time_recursive"},
		{nameIn: "container.blockio.io_serviced_recursive", nameOut: "io_serviced_recursive"},
		{nameIn: "container.blockio.io_time_recursive", nameOut: "io_time_recursive"},
		{nameIn: "container.blockio.io_wait_time_recursive", nameOut: "io_wait_time_recursive"},
		{nameIn: "container.blockio.sectors_recursive", nameOut: "sectors_recursive"},
	}

	for _, tc := range testcases {
		t.Run(tc.nameIn+"-"+tc.nameOut, func(t *testing.T) {
			actual := pmetric.NewMetric()
			actual.SetName(tc.nameIn)
			translateDockerMetric(actual)
			assert.Equal(t, tc.nameOut, actual.Name())
		})
	}
}

func TestTranslateDockerMetric_ResourceAttrbutesAreTranslatedCorrectly(t *testing.T) {
	testcases := []struct {
		nameIn  string
		nameOut string
	}{
		{nameIn: "container.id", nameOut: "container.FullID"},
		{nameIn: "container.image.name", nameOut: "container.ImageName"},
		{nameIn: "container.name", nameOut: "container.Name"},
	}

	for _, tc := range testcases {
		t.Run(tc.nameIn+"-"+tc.nameOut, func(t *testing.T) {
			actual := pcommon.NewMap()
			actual.PutStr(tc.nameIn, "a")
			translateDockerResourceAttributes(actual)

			res, ok := actual.Get(tc.nameOut)
			assert.True(t, ok)
			assert.Equal(t, res.AsString(), "a")
		})
	}
}
