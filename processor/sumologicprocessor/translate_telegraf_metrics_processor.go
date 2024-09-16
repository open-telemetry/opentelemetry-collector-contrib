// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// translateTelegrafMetricsProcessor translates metric names from OpenTelemetry to Sumo Logic convention
type translateTelegrafMetricsProcessor struct {
	shouldTranslate bool
}

// metricsTranslations maps Telegraf metric names to corresponding names in Sumo Logic convention
var metricsTranslations = map[string]string{
	// CPU metrics
	"cpu_usage_active":  "CPU_Total",
	"cpu_usage_idle":    "CPU_Idle",
	"cpu_usage_iowait":  "CPU_IOWait",
	"cpu_usage_irq":     "CPU_Irq",
	"cpu_usage_nice":    "CPU_Nice",
	"cpu_usage_softirq": "CPU_SoftIrq",
	"cpu_usage_steal":   "CPU_Stolen",
	"cpu_usage_System":  "CPU_Sys",
	"cpu_usage_user":    "CPU_User",
	"system_load1":      "CPU_LoadAvg_1min",
	"system_load5":      "CPU_LoadAvg_5min",
	"system_load15":     "CPU_LoadAvg_15min",

	// Disk metrics
	"disk_used":         "Disk_Used",
	"disk_used_percent": "Disk_UsedPercent",
	"disk_inodes_free":  "Disk_InodesAvailable",

	// Disk IO metrics
	"diskio_reads":       "Disk_Reads",
	"diskio_read_bytes":  "Disk_ReadBytes",
	"diskio_writes":      "Disk_Writes",
	"diskio_write_bytes": "Disk_WriteBytes",

	// Memory metrics
	"mem_total":             "Mem_Total",
	"mem_free":              "Mem_free",
	"mem_available":         "Mem_ActualFree",
	"mem_used":              "Mem_ActualUsed",
	"mem_used_percent":      "Mem_UsedPercent",
	"mem_available_percent": "Mem_FreePercent",

	// Procstat metrics
	"procstat_num_threads":  "Proc_Threads",
	"procstat_memory_vms":   "Proc_VMSize",
	"procstat_memory_rss":   "Proc_RSSize",
	"procstat_cpu_usage":    "Proc_CPU",
	"procstat_major_faults": "Proc_MajorFaults",
	"procstat_minor_faults": "Proc_MinorFaults",

	// Net metrics
	"net_bytes_sent":   "Net_OutBytes",
	"net_bytes_recv":   "Net_InBytes",
	"net_packets_sent": "Net_OutPackets",
	"net_packets_recv": "Net_InPackets",

	// Netstat metrics
	"netstat_tcp_close":       "TCP_Close",
	"netstat_tcp_close_wait":  "TCP_CloseWait",
	"netstat_tcp_closing":     "TCP_Closing",
	"netstat_tcp_established": "TCP_Established",
	"netstat_tcp_listen":      "TCP_Listen",
	"netstat_tcp_time_wait":   "TCP_TimeWait",
}

func newTranslateTelegrafMetricsProcessor(shouldTranslate bool) *translateTelegrafMetricsProcessor {
	return &translateTelegrafMetricsProcessor{
		shouldTranslate: shouldTranslate,
	}
}

func (proc *translateTelegrafMetricsProcessor) processLogs(_ plog.Logs) error {
	// No-op, this subprocessor doesn't process logs.
	return nil
}

func (proc *translateTelegrafMetricsProcessor) processMetrics(metrics pmetric.Metrics) error {
	if !proc.shouldTranslate {
		return nil
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			metricsSlice := rm.ScopeMetrics().At(j).Metrics()

			for k := 0; k < metricsSlice.Len(); k++ {
				translateTelegrafMetric(metricsSlice.At(k))
			}
		}
	}

	return nil
}

func (proc *translateTelegrafMetricsProcessor) processTraces(_ ptrace.Traces) error {
	// No-op, this subprocessor doesn't process traces.
	return nil
}

func (proc *translateTelegrafMetricsProcessor) isEnabled() bool {
	return proc.shouldTranslate
}

func (*translateTelegrafMetricsProcessor) ConfigPropertyName() string {
	return "translate_telegraf_attributes"
}

func translateTelegrafMetric(m pmetric.Metric) {
	name, exists := metricsTranslations[m.Name()]

	if exists {
		m.SetName(name)
	}
}
