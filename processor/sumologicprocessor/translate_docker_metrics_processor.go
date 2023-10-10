// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// translateTelegrafMetricsProcessor translates metric names from OpenTelemetry to Sumo Logic convention
type translateDockerMetricsProcessor struct {
	shouldTranslate bool
}

// metricsTranslations maps Telegraf metric names to corresponding names in Sumo Logic convention
var dockerMetricsTranslations = map[string]string{
	"container.cpu.percent":                           "cpu_percentage",
	"container.cpu.usage.system":                      "system_cpu_usage",
	"container.cpu.usage.percpu":                      "cpu_usage.percpu_usage",
	"container.cpu.usage.total":                       "cpu_usage.total_usage",
	"container.cpu.usage.kernelmode":                  "cpu_usage.usage_in_kernelmode",
	"container.cpu.usage.usermode":                    "cpu_usage.usage_in_usermode",
	"container.cpu.throttling_data.periods":           "throttling_data.periods",
	"container.cpu.throttling_data.throttled_periods": "throttling_data.throttled_periods",
	"container.cpu.throttling_data.throttled_time":    "throttling_data.throttled_time",
	"container.memory.usage.limit":                    "limit",
	"container.memory.usage.max":                      "max_usage",
	"container.memory.percent":                        "memory_percentage",
	"container.memory.usage.total":                    "usage",
	"container.memory.active_anon":                    "stats.active_anon",
	"container.memory.active_file":                    "stats.active_file",
	"container.memory.cache":                          "stats.cache",
	"container.memory.hierarchical_memory_limit":      "stats.hierarchical_memory_limit",
	"container.memory.inactive_anon":                  "stats.inactive_anon",
	"container.memory.inactive_file":                  "stats.inactive_file",
	"container.memory.mapped_file":                    "stats.mapped_file",
	"container.memory.pgfault":                        "stats.pgfault",
	"container.memory.pgmajfault":                     "stats.pgmajfault",
	"container.memory.pgpgin":                         "stats.pgpgin",
	"container.memory.pgpgout":                        "stats.pgpgout",
	"container.memory.rss":                            "stats.rss",
	"container.memory.rss_huge":                       "stats.rss_huge",
	"container.memory.unevictable":                    "stats.unevictable",
	"container.memory.writeback":                      "stats.writeback",
	"container.memory.total_active_anon":              "stats.total_active_anon",
	"container.memory.total_active_file":              "stats.total_active_file",
	"container.memory.total_cache":                    "stats.total_cache",
	"container.memory.total_inactive_anon":            "stats.total_inactive_anon",
	"container.memory.total_mapped_file":              "stats.total_mapped_file",
	"container.memory.total_pgfault":                  "stats.total_pgfault",
	"container.memory.total_pgmajfault":               "stats.total_pgmajfault",
	"container.memory.total_pgpgin":                   "stats.total_pgpgin",
	"container.memory.total_pgpgout":                  "stats.total_pgpgout",
	"container.memory.total_rss":                      "stats.total_rss",
	"container.memory.total_rss_huge":                 "stats.total_rss_huge",
	"container.memory.total_unevictable":              "stats.total_unevictable",
	"container.memory.total_writeback":                "stats.total_writeback",
	"container.blockio.io_merged_recursive":           "io_merged_recursive",
	"container.blockio.io_queued_recursive":           "io_queue_recursive",
	"container.blockio.io_service_bytes_recursive":    "io_service_bytes_recursive",
	"container.blockio.io_service_time_recursive":     "io_service_time_recursive",
	"container.blockio.io_serviced_recursive":         "io_serviced_recursive",
	"container.blockio.io_time_recursive":             "io_time_recursive",
	"container.blockio.io_wait_time_recursive":        "io_wait_time_recursive",
	"container.blockio.sectors_recursive":             "sectors_recursive",
}

var dockerReasourceAttributeTranslations = map[string]string{
	"container.id":         "container.FullID",
	"container.image.name": "container.ImageName",
	"container.name":       "container.Name",
}

func newTranslateDockerMetricsProcessor(shouldTranslate bool) *translateDockerMetricsProcessor {
	return &translateDockerMetricsProcessor{
		shouldTranslate: shouldTranslate,
	}
}

func (proc *translateDockerMetricsProcessor) processLogs(_ plog.Logs) error {
	// No-op, this subprocessor doesn't process logs.
	return nil
}

func (proc *translateDockerMetricsProcessor) processMetrics(metrics pmetric.Metrics) error {
	if !proc.shouldTranslate {
		return nil
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		translateDockerResourceAttributes(rm.Resource().Attributes())

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			metricsSlice := rm.ScopeMetrics().At(j).Metrics()

			for k := 0; k < metricsSlice.Len(); k++ {
				translateDockerMetric(metricsSlice.At(k))
			}
		}
	}

	return nil
}

func (proc *translateDockerMetricsProcessor) processTraces(_ ptrace.Traces) error {
	// No-op, this subprocessor doesn't process traces.
	return nil
}

func (proc *translateDockerMetricsProcessor) isEnabled() bool {
	return proc.shouldTranslate
}

func (*translateDockerMetricsProcessor) ConfigPropertyName() string {
	return "translate_docker_metrics"
}

func translateDockerMetric(m pmetric.Metric) {
	name, exists := dockerMetricsTranslations[m.Name()]

	if exists {
		m.SetName(name)
	}
}

func translateDockerResourceAttributes(attributes pcommon.Map) {
	result := pcommon.NewMap()
	result.EnsureCapacity(attributes.Len())

	attributes.Range(func(otKey string, value pcommon.Value) bool {
		if sumoKey, ok := dockerReasourceAttributeTranslations[otKey]; ok {
			// Only insert if it doesn't exist yet to prevent overwriting.
			// We have to do it this way since the final return value is not
			// ready yet to rely on .Insert() not overwriting.
			if _, exists := attributes.Get(sumoKey); !exists {
				if _, ok := result.Get(sumoKey); !ok {
					value.CopyTo(result.PutEmpty(sumoKey))
				}
			} else {
				if _, ok := result.Get(otKey); !ok {
					value.CopyTo(result.PutEmpty(otKey))
				}
			}
		} else {
			if _, ok := result.Get(otKey); !ok {
				value.CopyTo(result.PutEmpty(otKey))
			}
		}
		return true
	})

	result.CopyTo(attributes)
}
