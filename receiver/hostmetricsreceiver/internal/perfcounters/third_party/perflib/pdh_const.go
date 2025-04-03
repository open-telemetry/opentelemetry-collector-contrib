//go:build windows

// Constants copied from:
// https://github.com/prometheus-community/windows_exporter/tree/4c7df1ccaf65df8662a2d67a7eff93019c94d98c/internal/pdh
// Required for perflib functionality.

package perflib // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters/third_party/perflib"

// Based on https://github.com/leoluk/perflib_exporter/blob/master/collector/mapper.go
//
//goland:noinspection GoUnusedConst
const (
	PERF_COUNTER_RAWCOUNT_HEX           = 0x00000000
	PERF_COUNTER_LARGE_RAWCOUNT_HEX     = 0x00000100
	PERF_COUNTER_TEXT                   = 0x00000b00
	PERF_COUNTER_RAWCOUNT               = 0x00010000
	PERF_COUNTER_LARGE_RAWCOUNT         = 0x00010100
	PERF_DOUBLE_RAW                     = 0x00012000
	PERF_COUNTER_DELTA                  = 0x00400400
	PERF_COUNTER_LARGE_DELTA            = 0x00400500
	PERF_SAMPLE_COUNTER                 = 0x00410400
	PERF_COUNTER_QUEUELEN_TYPE          = 0x00450400
	PERF_COUNTER_LARGE_QUEUELEN_TYPE    = 0x00450500
	PERF_COUNTER_100NS_QUEUELEN_TYPE    = 0x00550500
	PERF_COUNTER_OBJ_TIME_QUEUELEN_TYPE = 0x00650500
	PERF_COUNTER_COUNTER                = 0x10410400
	PERF_COUNTER_BULK_COUNT             = 0x10410500
	PERF_RAW_FRACTION                   = 0x20020400
	PERF_LARGE_RAW_FRACTION             = 0x20020500
	PERF_COUNTER_TIMER                  = 0x20410500
	PERF_PRECISION_SYSTEM_TIMER         = 0x20470500
	PERF_100NSEC_TIMER                  = 0x20510500
	PERF_PRECISION_100NS_TIMER          = 0x20570500
	PERF_OBJ_TIME_TIMER                 = 0x20610500
	PERF_PRECISION_OBJECT_TIMER         = 0x20670500
	PERF_SAMPLE_FRACTION                = 0x20c20400
	PERF_COUNTER_TIMER_INV              = 0x21410500
	PERF_100NSEC_TIMER_INV              = 0x21510500
	PERF_COUNTER_MULTI_TIMER            = 0x22410500
	PERF_100NSEC_MULTI_TIMER            = 0x22510500
	PERF_COUNTER_MULTI_TIMER_INV        = 0x23410500
	PERF_100NSEC_MULTI_TIMER_INV        = 0x23510500
	PERF_AVERAGE_TIMER                  = 0x30020400
	PERF_ELAPSED_TIME                   = 0x30240500
	PERF_COUNTER_NODATA                 = 0x40000200
	PERF_AVERAGE_BULK                   = 0x40020500
	PERF_SAMPLE_BASE                    = 0x40030401
	PERF_AVERAGE_BASE                   = 0x40030402
	PERF_RAW_BASE                       = 0x40030403
	PERF_PRECISION_TIMESTAMP            = 0x40030500
	PERF_LARGE_RAW_BASE                 = 0x40030503
	PERF_COUNTER_MULTI_BASE             = 0x42030500
	PERF_COUNTER_HISTOGRAM_TYPE         = 0x80000000
)

// Conversion factors.
const (
	TicksToSecondScaleFactor       = 1 / 1e7
	WindowsEpoch             int64 = 116444736000000000
)

const (
	InstanceEmpty = "------"
	InstanceTotal = "_Total"
)
