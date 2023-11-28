// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestTranslateTelegrafMetric_NamesAreTranslatedCorrectly(t *testing.T) {
	testcases := []struct {
		nameIn  string
		nameOut string
	}{
		// CPU metrics
		{
			nameIn:  "cpu_usage_active",
			nameOut: "CPU_Total",
		},
		{
			nameIn:  "cpu_usage_idle",
			nameOut: "CPU_Idle",
		},
		{
			nameIn:  "cpu_usage_iowait",
			nameOut: "CPU_IOWait",
		},
		{
			nameIn:  "cpu_usage_irq",
			nameOut: "CPU_Irq",
		},
		{
			nameIn:  "cpu_usage_nice",
			nameOut: "CPU_Nice",
		},
		{
			nameIn:  "cpu_usage_softirq",
			nameOut: "CPU_SoftIrq",
		},
		{
			nameIn:  "cpu_usage_steal",
			nameOut: "CPU_Stolen",
		},
		{
			nameIn:  "cpu_usage_System",
			nameOut: "CPU_Sys",
		},
		{
			nameIn:  "cpu_usage_user",
			nameOut: "CPU_User",
		},
		{
			nameIn:  "system_load1",
			nameOut: "CPU_LoadAvg_1min",
		},
		{
			nameIn:  "system_load5",
			nameOut: "CPU_LoadAvg_5min",
		},
		{
			nameIn:  "system_load15",
			nameOut: "CPU_LoadAvg_15min",
		},
		// Disk metrics
		{
			nameIn:  "disk_used",
			nameOut: "Disk_Used",
		},
		{
			nameIn:  "disk_used_percent",
			nameOut: "Disk_UsedPercent",
		},
		{
			nameIn:  "disk_inodes_free",
			nameOut: "Disk_InodesAvailable",
		},
		// Disk IO metrics
		{
			nameIn:  "diskio_reads",
			nameOut: "Disk_Reads",
		},
		{
			nameIn:  "diskio_read_bytes",
			nameOut: "Disk_ReadBytes",
		},
		{
			nameIn:  "diskio_writes",
			nameOut: "Disk_Writes",
		},
		{
			nameIn:  "diskio_write_bytes",
			nameOut: "Disk_WriteBytes",
		},

		// Memory metrics
		{
			nameIn:  "mem_total",
			nameOut: "Mem_Total",
		},
		{
			nameIn:  "mem_free",
			nameOut: "Mem_free",
		},
		{
			nameIn:  "mem_available",
			nameOut: "Mem_ActualFree",
		},
		{
			nameIn:  "mem_used",
			nameOut: "Mem_ActualUsed",
		},
		{
			nameIn:  "mem_used_percent",
			nameOut: "Mem_UsedPercent",
		},
		{
			nameIn:  "mem_available_percent",
			nameOut: "Mem_FreePercent",
		},

		// Procstat metrics
		{
			nameIn:  "procstat_num_threads",
			nameOut: "Proc_Threads",
		},
		{
			nameIn:  "procstat_memory_vms",
			nameOut: "Proc_VMSize",
		},
		{
			nameIn:  "procstat_memory_rss",
			nameOut: "Proc_RSSize",
		},
		{
			nameIn:  "procstat_cpu_usage",
			nameOut: "Proc_CPU",
		},
		{
			nameIn:  "procstat_major_faults",
			nameOut: "Proc_MajorFaults",
		},
		{
			nameIn:  "procstat_minor_faults",
			nameOut: "Proc_MinorFaults",
		},

		// Net metrics
		{
			nameIn:  "net_bytes_sent",
			nameOut: "Net_OutBytes",
		},
		{
			nameIn:  "net_bytes_recv",
			nameOut: "Net_InBytes",
		},
		{
			nameIn:  "net_packets_sent",
			nameOut: "Net_OutPackets",
		},
		{
			nameIn:  "net_packets_recv",
			nameOut: "Net_InPackets",
		},

		// Netstat metrics
		{
			nameIn:  "netstat_tcp_close",
			nameOut: "TCP_Close",
		},
		{
			nameIn:  "netstat_tcp_close_wait",
			nameOut: "TCP_CloseWait",
		},
		{
			nameIn:  "netstat_tcp_closing",
			nameOut: "TCP_Closing",
		},
		{
			nameIn:  "netstat_tcp_established",
			nameOut: "TCP_Established",
		},
		{
			nameIn:  "netstat_tcp_listen",
			nameOut: "TCP_Listen",
		},
		{
			nameIn:  "netstat_tcp_time_wait",
			nameOut: "TCP_TimeWait",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.nameIn+"-"+tc.nameOut, func(t *testing.T) {
			actual := pmetric.NewMetric()
			actual.SetName(tc.nameIn)
			translateTelegrafMetric(actual)
			assert.Equal(t, tc.nameOut, actual.Name())
		})
	}
}
