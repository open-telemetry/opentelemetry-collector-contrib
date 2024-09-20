// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"

var DefaultTranslationRules = []Rule{
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			// kubeletstats container cpu needed for calculation below
			"container.cpu.time": "sf_temp.container_cpu_utilization",
		},
	},

	// compute cpu utilization metrics: cpu.utilization_per_core (excluded by default) and cpu.utilization
	{
		Action: ActionDeltaMetric,
		Mapping: map[string]string{
			"system.cpu.time": "sf_temp.system.cpu.delta",
		},
	},
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"sf_temp.system.cpu.delta": "sf_temp.system.cpu.usage",
		},
		DimensionKey: "state",
		DimensionValues: map[string]bool{
			"interrupt": true,
			"nice":      true,
			"softirq":   true,
			"steal":     true,
			"system":    true,
			"user":      true,
			"wait":      true,
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.system.cpu.usage",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"state",
		},
	},
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"sf_temp.system.cpu.delta": "sf_temp.system.cpu.total",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.system.cpu.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"state",
		},
	},
	{
		Action:         ActionCalculateNewMetric,
		MetricName:     "cpu.utilization_per_core",
		Operand1Metric: "sf_temp.system.cpu.usage",
		Operand2Metric: "sf_temp.system.cpu.total",
		Operator:       "/",
	},
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"cpu.utilization_per_core": "sf_temp.cpu.utilization",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.utilization",
		AggregationMethod: "avg",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action: ActionMultiplyFloat,
		ScaleFactorsFloat: map[string]float64{
			"sf_temp.cpu.utilization": 100,
		},
	},

	// convert cpu metrics
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.cpu.time": "sf_temp.system.cpu.time",
		},
	},
	{
		Action:       ActionSplitMetric,
		MetricName:   "sf_temp.system.cpu.time",
		DimensionKey: "state",
		Mapping: map[string]string{
			"idle":      "sf_temp.cpu.idle",
			"interrupt": "sf_temp.cpu.interrupt",
			"system":    "sf_temp.cpu.system",
			"user":      "sf_temp.cpu.user",
			"steal":     "sf_temp.cpu.steal",
			"wait":      "sf_temp.cpu.wait",
			"softirq":   "sf_temp.cpu.softirq",
			"nice":      "sf_temp.cpu.nice",
		},
	},
	{
		Action: ActionMultiplyFloat,
		ScaleFactorsFloat: map[string]float64{
			"sf_temp.container_cpu_utilization": 100,
			"sf_temp.cpu.idle":                  100,
			"sf_temp.cpu.interrupt":             100,
			"sf_temp.cpu.system":                100,
			"sf_temp.cpu.user":                  100,
			"sf_temp.cpu.steal":                 100,
			"sf_temp.cpu.wait":                  100,
			"sf_temp.cpu.softirq":               100,
			"sf_temp.cpu.nice":                  100,
		},
	},
	{
		Action: ActionConvertValues,
		TypesMapping: map[string]MetricValueType{
			"sf_temp.container_cpu_utilization": "int",
			"sf_temp.cpu.idle":                  "int",
			"sf_temp.cpu.interrupt":             "int",
			"sf_temp.cpu.system":                "int",
			"sf_temp.cpu.user":                  "int",
			"sf_temp.cpu.steal":                 "int",
			"sf_temp.cpu.wait":                  "int",
			"sf_temp.cpu.softirq":               "int",
			"sf_temp.cpu.nice":                  "int",
		},
	},

	// compute cpu.num_processors
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"sf_temp.cpu.idle": "sf_temp.cpu.num_processors",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.num_processors",
		AggregationMethod: "count",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"sf_temp.cpu.idle":      "sf_temp.cpu.idle_per_core",
			"sf_temp.cpu.interrupt": "sf_temp.cpu.interrupt_per_core",
			"sf_temp.cpu.system":    "sf_temp.cpu.system_per_core",
			"sf_temp.cpu.user":      "sf_temp.cpu.user_per_core",
			"sf_temp.cpu.wait":      "sf_temp.cpu.wait_per_core",
			"sf_temp.cpu.steal":     "sf_temp.cpu.steal_per_core",
			"sf_temp.cpu.softirq":   "sf_temp.cpu.softirq_per_core",
			"sf_temp.cpu.nice":      "sf_temp.cpu.nice_per_core",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.idle",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.interrupt",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.system",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.user",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.steal",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.wait",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.softirq",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.cpu.nice",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"cpu",
		},
	},

	// compute memory.total
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.memory.usage": "sf_temp.memory.total",
		},
		DimensionKey: "state",
		DimensionValues: map[string]bool{
			"buffered": true,
			"cached":   true,
			"free":     true,
			"used":     true,
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.memory.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"state",
		},
	},

	// convert memory metrics
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.memory.usage": "sf_temp.system.memory.usage",
		},
	},

	// sf_temp.memory.used needed to calculate memory.utilization
	{
		Action:       ActionSplitMetric,
		MetricName:   "sf_temp.system.memory.usage",
		DimensionKey: "state",
		Mapping: map[string]string{
			"used": "sf_temp.memory.used",
		},
	},

	// Translations to derive filesystem metrics
	// sf_temp.disk.total, required to compute disk.utilization
	// same as df, disk.utilization = (used/(used + free)) * 100 see: https://github.com/shirou/gopsutil/issues/562
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.filesystem.usage": "sf_temp.disk.total",
		},
		DimensionKey: "state",
		DimensionValues: map[string]bool{
			"used": true,
			"free": true,
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.disk.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"state",
		},
	},

	// same as df, don't count root fs, ie: total = used + free
	// sf_temp.disk.summary_total, required to compute disk.summary_utilization
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.filesystem.usage": "sf_temp.disk.summary_total",
		},
		DimensionKey: "state",
		DimensionValues: map[string]bool{
			"used": true,
			"free": true,
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.disk.summary_total",
		AggregationMethod: "avg",
		WithoutDimensions: []string{
			"mode",
			"mountpoint",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.disk.summary_total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"state",
			"device",
			"type",
		},
	},

	// sf_temp.df_complex.used needed to calculate disk.utilization
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.filesystem.usage": "sf_temp.system.filesystem.usage",
		},
	},
	{
		Action:       ActionSplitMetric,
		MetricName:   "sf_temp.system.filesystem.usage",
		DimensionKey: "state",
		Mapping: map[string]string{
			"used": "sf_temp.df_complex.used",
		},
	},

	// disk.utilization
	{
		Action:         ActionCalculateNewMetric,
		MetricName:     "sf_temp.disk.utilization",
		Operand1Metric: "sf_temp.df_complex.used",
		Operand2Metric: "sf_temp.disk.total",
		Operator:       "/",
	},
	{
		Action: ActionMultiplyFloat,
		ScaleFactorsFloat: map[string]float64{
			"sf_temp.disk.utilization": 100,
		},
	},

	// disk.summary_utilization
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"sf_temp.df_complex.used": "sf_temp.df_complex.used_total",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.df_complex.used_total",
		AggregationMethod: "avg",
		WithoutDimensions: []string{
			"mode",
			"mountpoint",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.df_complex.used_total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"device",
			"type",
		},
	},
	{
		Action:         ActionCalculateNewMetric,
		MetricName:     "sf_temp.disk.summary_utilization",
		Operand1Metric: "sf_temp.df_complex.used_total",
		Operand2Metric: "sf_temp.disk.summary_total",
		Operator:       "/",
	},
	{
		Action: ActionMultiplyFloat,
		ScaleFactorsFloat: map[string]float64{
			"sf_temp.disk.summary_utilization": 100,
		},
	},

	// Translations to derive disk I/O metrics.
	// Calculate extra system.disk.operations.total and system.disk.io.total metrics summing up read/write ops/IO across all devices.
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.disk.operations": "sf_temp.system.disk.operations.total",
			"system.disk.io":         "sf_temp.system.disk.io.total",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.system.disk.operations.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"device",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.system.disk.io.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"device",
		},
	},

	// Calculate an extra disk_ops.total metric as number all all read and write operations happened since the last report.
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.disk.operations": "sf_temp.disk.ops",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.disk.ops",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"direction",
			"device",
		},
	},
	{
		Action: ActionDeltaMetric,
		Mapping: map[string]string{
			"sf_temp.disk.ops": "disk_ops.total",
		},
	},
	{
		Action: ActionDeltaMetric,
		Mapping: map[string]string{
			"system.disk.pending_operations": "disk_ops.pending",
		},
	},

	// Translations to derive Network I/O metrics
	// Calculate extra network I/O metrics system.network.packets.total and system.network.io.total.
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.network.packets": "sf_temp.system.network.packets.total",
			"system.network.io":      "sf_temp.system.network.io.total",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.system.network.packets.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"device",
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.system.network.io.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"device",
		},
	},

	// Calculate extra network.total metric.
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.network.io": "sf_temp.network.total",
		},
		DimensionKey: "direction",
		DimensionValues: map[string]bool{
			"receive":  true,
			"transmit": true,
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.network.total",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"direction",
			"device",
		},
	},

	// memory utilization
	{
		Action:         ActionCalculateNewMetric,
		MetricName:     "sf_temp.memory.utilization",
		Operand1Metric: "sf_temp.memory.used",
		Operand2Metric: "sf_temp.memory.total",
		Operator:       "/",
	},
	{
		Action: ActionMultiplyFloat,
		ScaleFactorsFloat: map[string]float64{
			"sf_temp.memory.utilization": 100,
		},
	},

	// Virtual memory metrics
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"system.paging.operations": "sf_temp.system.paging.operations",
		},
	},
	{
		Action:       ActionSplitMetric,
		MetricName:   "sf_temp.system.paging.operations",
		DimensionKey: "direction",
		Mapping: map[string]string{
			"page_in":  "sf_temp.system.paging.operations.page_in",
			"page_out": "sf_temp.system.paging.operations.page_out",
		},
	},
	{
		Action:       ActionSplitMetric,
		MetricName:   "sf_temp.system.paging.operations.page_in",
		DimensionKey: "type",
		Mapping: map[string]string{
			"major": "vmpage_io.swap.in",
			"minor": "vmpage_io.memory.in",
		},
	},
	{
		Action:       ActionSplitMetric,
		MetricName:   "sf_temp.system.paging.operations.page_out",
		DimensionKey: "type",
		Mapping: map[string]string{
			"major": "vmpage_io.swap.out",
			"minor": "vmpage_io.memory.out",
		},
	},

	// convert from bytes to pages
	{
		Action: ActionDivideInt,
		ScaleFactorsInt: map[string]int64{
			"vmpage_io.swap.in":    4096,
			"vmpage_io.swap.out":   4096,
			"vmpage_io.memory.in":  4096,
			"vmpage_io.memory.out": 4096,
		},
	},

	// process metric
	{
		Action: ActionCopyMetrics,
		Mapping: map[string]string{
			"process.cpu.time": "sf_temp.process.cpu.time",
		},
		DimensionKey: "state",
		DimensionValues: map[string]bool{
			"user":   true,
			"system": true,
		},
	},
	{
		Action:            ActionAggregateMetric,
		MetricName:        "sf_temp.process.cpu.time",
		AggregationMethod: "sum",
		WithoutDimensions: []string{
			"state",
		},
	},
	{
		Action: ActionRenameMetrics,
		Mapping: map[string]string{
			"sf_temp.container_cpu_utilization":    "container_cpu_utilization",
			"sf_temp.cpu.idle":                     "cpu.idle",
			"sf_temp.cpu.idle_per_core":            "cpu.idle",
			"sf_temp.cpu.interrupt":                "cpu.interrupt",
			"sf_temp.cpu.interrupt_per_core":       "cpu.interrupt",
			"sf_temp.cpu.nice":                     "cpu.nice",
			"sf_temp.cpu.nice_per_core":            "cpu.nice",
			"sf_temp.cpu.num_processors":           "cpu.num_processors",
			"sf_temp.cpu.softirq":                  "cpu.softirq",
			"sf_temp.cpu.softirq_per_core":         "cpu.softirq",
			"sf_temp.cpu.steal":                    "cpu.steal",
			"sf_temp.cpu.steal_per_core":           "cpu.steal",
			"sf_temp.cpu.system":                   "cpu.system",
			"sf_temp.cpu.system_per_core":          "cpu.system",
			"sf_temp.cpu.user":                     "cpu.user",
			"sf_temp.cpu.user_per_core":            "cpu.user",
			"sf_temp.cpu.utilization":              "cpu.utilization",
			"sf_temp.cpu.wait":                     "cpu.wait",
			"sf_temp.cpu.wait_per_core":            "cpu.wait",
			"sf_temp.disk.summary_utilization":     "disk.summary_utilization",
			"sf_temp.disk.utilization":             "disk.utilization",
			"sf_temp.memory.total":                 "memory.total",
			"sf_temp.memory.utilization":           "memory.utilization",
			"sf_temp.network.total":                "network.total",
			"sf_temp.system.disk.io.total":         "system.disk.io.total",
			"sf_temp.system.disk.operations.total": "system.disk.operations.total",
			"sf_temp.system.network.io.total":      "system.network.io.total",
			"sf_temp.system.network.packets.total": "system.network.packets.total",
			"sf_temp.process.cpu.time":             "process.cpu_time_seconds",
		},
	},

	// remove redundant metrics
	{
		Action: ActionDropMetrics,
		MetricNames: map[string]bool{
			"sf_temp.df_complex.used":                   true,
			"sf_temp.df_complex.used_total":             true,
			"sf_temp.disk.ops":                          true,
			"sf_temp.disk.summary_total":                true,
			"sf_temp.disk.total":                        true,
			"sf_temp.memory.used":                       true,
			"sf_temp.system.cpu.delta":                  true,
			"sf_temp.system.cpu.total":                  true,
			"sf_temp.system.cpu.time":                   true,
			"sf_temp.system.cpu.usage":                  true,
			"sf_temp.system.filesystem.usage":           true,
			"sf_temp.system.memory.usage":               true,
			"sf_temp.system.paging.operations":          true,
			"sf_temp.system.paging.operations.page_in":  true,
			"sf_temp.system.paging.operations.page_out": true,
		},
	},
}

// Note: This is now only used for testing to ensure the movement of default
// translation rules had no functional impact.
const OriginalDefaultTranslationRulesYaml = `
translation_rules:
- action: copy_metrics
  mapping:
    # kubeletstats container cpu needed for calculation below
    container.cpu.time: sf_temp.container_cpu_utilization

# compute cpu utilization metrics: cpu.utilization_per_core (excluded by default) and cpu.utilization
- action: delta_metric
  mapping:
    system.cpu.time: sf_temp.system.cpu.delta
- action: copy_metrics
  mapping:
    sf_temp.system.cpu.delta: sf_temp.system.cpu.usage
  dimension_key: state
  dimension_values:
    interrupt: true
    nice: true
    softirq: true
    steal: true
    system: true
    user: true
    wait: true
- action: aggregate_metric
  metric_name: sf_temp.system.cpu.usage
  aggregation_method: sum
  without_dimensions:
  - state
- action: copy_metrics
  mapping:
    sf_temp.system.cpu.delta: sf_temp.system.cpu.total
- action: aggregate_metric
  metric_name: sf_temp.system.cpu.total
  aggregation_method: sum
  without_dimensions:
  - state
- action: calculate_new_metric
  metric_name: cpu.utilization_per_core
  operand1_metric: sf_temp.system.cpu.usage
  operand2_metric: sf_temp.system.cpu.total
  operator: /
- action: copy_metrics
  mapping:
    cpu.utilization_per_core: sf_temp.cpu.utilization
- action: aggregate_metric
  metric_name: sf_temp.cpu.utilization
  aggregation_method: avg
  without_dimensions:
  - cpu
- action: multiply_float
  scale_factors_float:
    sf_temp.cpu.utilization: 100

# convert cpu metrics
- action: copy_metrics
  mapping:
    system.cpu.time: sf_temp.system.cpu.time
- action: split_metric
  metric_name: sf_temp.system.cpu.time
  dimension_key: state
  mapping:
    idle: sf_temp.cpu.idle
    interrupt: sf_temp.cpu.interrupt
    system: sf_temp.cpu.system
    user: sf_temp.cpu.user
    steal: sf_temp.cpu.steal
    wait: sf_temp.cpu.wait
    softirq: sf_temp.cpu.softirq
    nice: sf_temp.cpu.nice
- action: multiply_float
  scale_factors_float:
    sf_temp.container_cpu_utilization: 100
    sf_temp.cpu.idle: 100
    sf_temp.cpu.interrupt: 100
    sf_temp.cpu.system: 100
    sf_temp.cpu.user: 100
    sf_temp.cpu.steal: 100
    sf_temp.cpu.wait: 100
    sf_temp.cpu.softirq: 100
    sf_temp.cpu.nice: 100
- action: convert_values
  types_mapping:
    sf_temp.container_cpu_utilization: int
    sf_temp.cpu.idle: int
    sf_temp.cpu.interrupt: int
    sf_temp.cpu.system: int
    sf_temp.cpu.user: int
    sf_temp.cpu.steal: int
    sf_temp.cpu.wait: int
    sf_temp.cpu.softirq: int
    sf_temp.cpu.nice: int

# compute cpu.num_processors
- action: copy_metrics
  mapping:
    sf_temp.cpu.idle: sf_temp.cpu.num_processors
- action: aggregate_metric
  metric_name: sf_temp.cpu.num_processors
  aggregation_method: count
  without_dimensions:
  - cpu

- action: copy_metrics
  mapping:
    sf_temp.cpu.idle: sf_temp.cpu.idle_per_core
    sf_temp.cpu.interrupt: sf_temp.cpu.interrupt_per_core
    sf_temp.cpu.system: sf_temp.cpu.system_per_core
    sf_temp.cpu.user: sf_temp.cpu.user_per_core
    sf_temp.cpu.wait: sf_temp.cpu.wait_per_core
    sf_temp.cpu.steal: sf_temp.cpu.steal_per_core
    sf_temp.cpu.softirq: sf_temp.cpu.softirq_per_core
    sf_temp.cpu.nice: sf_temp.cpu.nice_per_core

- action: aggregate_metric
  metric_name: sf_temp.cpu.idle
  aggregation_method: sum
  without_dimensions:
  - cpu
- action: aggregate_metric
  metric_name: sf_temp.cpu.interrupt
  aggregation_method: sum
  without_dimensions:
  - cpu
- action: aggregate_metric
  metric_name: sf_temp.cpu.system
  aggregation_method: sum
  without_dimensions:
  - cpu
- action: aggregate_metric
  metric_name: sf_temp.cpu.user
  aggregation_method: sum
  without_dimensions:
  - cpu
- action: aggregate_metric
  metric_name: sf_temp.cpu.steal
  aggregation_method: sum
  without_dimensions:
  - cpu
- action: aggregate_metric
  metric_name: sf_temp.cpu.wait
  aggregation_method: sum
  without_dimensions:
  - cpu
- action: aggregate_metric
  metric_name: sf_temp.cpu.softirq
  aggregation_method: sum
  without_dimensions:
  - cpu
- action: aggregate_metric
  metric_name: sf_temp.cpu.nice
  aggregation_method: sum
  without_dimensions:
  - cpu

# compute memory.total
- action: copy_metrics
  mapping:
    system.memory.usage: sf_temp.memory.total
  dimension_key: state
  dimension_values:
    buffered: true
    cached: true
    free: true
    used: true
- action: aggregate_metric
  metric_name: sf_temp.memory.total
  aggregation_method: sum
  without_dimensions:
  - state

# convert memory metrics
- action: copy_metrics
  mapping:
    system.memory.usage: sf_temp.system.memory.usage

# sf_temp.memory.used needed to calculate memory.utilization
- action: split_metric
  metric_name: sf_temp.system.memory.usage
  dimension_key: state
  mapping:
    used: sf_temp.memory.used

# Translations to derive filesystem metrics
## sf_temp.disk.total, required to compute disk.utilization
## same as df, disk.utilization = (used/(used + free)) * 100 see: https://github.com/shirou/gopsutil/issues/562
- action: copy_metrics
  mapping:
    system.filesystem.usage: sf_temp.disk.total
  dimension_key: state
  dimension_values:
    used: true
    free: true
- action: aggregate_metric
  metric_name: sf_temp.disk.total
  aggregation_method: sum
  without_dimensions:
    - state

## sf_temp.disk.summary_total, required to compute disk.summary_utilization
## same as df, don't count root fs, ie: total = used + free
- action: copy_metrics
  mapping:
    system.filesystem.usage: sf_temp.disk.summary_total
  dimension_key: state
  dimension_values:
    used: true
    free: true
- action: aggregate_metric
  metric_name: sf_temp.disk.summary_total
  aggregation_method: avg
  without_dimensions:
    - mode
    - mountpoint
- action: aggregate_metric
  metric_name: sf_temp.disk.summary_total
  aggregation_method: sum
  without_dimensions:
    - state
    - device
    - type

## sf_temp.df_complex.used needed to calculate disk.utilization
- action: copy_metrics
  mapping:
    system.filesystem.usage: sf_temp.system.filesystem.usage

- action: split_metric
  metric_name: sf_temp.system.filesystem.usage
  dimension_key: state
  mapping:
    used: sf_temp.df_complex.used

## disk.utilization
- action: calculate_new_metric
  metric_name: sf_temp.disk.utilization
  operand1_metric: sf_temp.df_complex.used
  operand2_metric: sf_temp.disk.total
  operator: /
- action: multiply_float
  scale_factors_float:
    sf_temp.disk.utilization: 100

## disk.summary_utilization
- action: copy_metrics
  mapping:
    sf_temp.df_complex.used: sf_temp.df_complex.used_total

- action: aggregate_metric
  metric_name: sf_temp.df_complex.used_total
  aggregation_method: avg
  without_dimensions:
    - mode
    - mountpoint

- action: aggregate_metric
  metric_name: sf_temp.df_complex.used_total
  aggregation_method: sum
  without_dimensions:
  - device
  - type

- action: calculate_new_metric
  metric_name: sf_temp.disk.summary_utilization
  operand1_metric: sf_temp.df_complex.used_total
  operand2_metric: sf_temp.disk.summary_total
  operator: /
- action: multiply_float
  scale_factors_float:
    sf_temp.disk.summary_utilization: 100


# Translations to derive disk I/O metrics.

## Calculate extra system.disk.operations.total and system.disk.io.total metrics summing up read/write ops/IO across all devices.
- action: copy_metrics
  mapping:
    system.disk.operations: sf_temp.system.disk.operations.total
    system.disk.io: sf_temp.system.disk.io.total
- action: aggregate_metric
  metric_name: sf_temp.system.disk.operations.total
  aggregation_method: sum
  without_dimensions:
    - device
- action: aggregate_metric
  metric_name: sf_temp.system.disk.io.total
  aggregation_method: sum
  without_dimensions:
    - device

## Calculate an extra disk_ops.total metric as number of all read and write operations happened since the last report.
- action: copy_metrics
  mapping:
    system.disk.operations: sf_temp.disk.ops
- action: aggregate_metric
  metric_name: sf_temp.disk.ops
  aggregation_method: sum
  without_dimensions:
    - direction
    - device
- action: delta_metric
  mapping:
    sf_temp.disk.ops: disk_ops.total

- action: delta_metric
  mapping:
    system.disk.pending_operations: disk_ops.pending

# Translations to derive Network I/O metrics.

## Calculate extra network I/O metrics system.network.packets.total and system.network.io.total.
- action: copy_metrics
  mapping:
    system.network.packets: sf_temp.system.network.packets.total
    system.network.io: sf_temp.system.network.io.total
- action: aggregate_metric
  metric_name: sf_temp.system.network.packets.total
  aggregation_method: sum
  without_dimensions:
  - device
- action: aggregate_metric
  metric_name: sf_temp.system.network.io.total
  aggregation_method: sum
  without_dimensions:
  - device

## Calculate extra network.total metric.
- action: copy_metrics
  mapping:
    system.network.io: sf_temp.network.total
  dimension_key: direction
  dimension_values:
    receive: true
    transmit: true
- action: aggregate_metric
  metric_name: sf_temp.network.total
  aggregation_method: sum
  without_dimensions:
  - direction
  - device

# memory utilization
- action: calculate_new_metric
  metric_name: sf_temp.memory.utilization
  operand1_metric: sf_temp.memory.used
  operand2_metric: sf_temp.memory.total
  operator: /

- action: multiply_float
  scale_factors_float:
    sf_temp.memory.utilization: 100

# Virtual memory metrics
- action: copy_metrics
  mapping:
    system.paging.operations: sf_temp.system.paging.operations
- action: split_metric
  metric_name: sf_temp.system.paging.operations
  dimension_key: direction
  mapping:
    page_in: sf_temp.system.paging.operations.page_in
    page_out: sf_temp.system.paging.operations.page_out

- action: split_metric
  metric_name: sf_temp.system.paging.operations.page_in
  dimension_key: type
  mapping:
    major: vmpage_io.swap.in
    minor: vmpage_io.memory.in

- action: split_metric
  metric_name: sf_temp.system.paging.operations.page_out
  dimension_key: type
  mapping:
    major: vmpage_io.swap.out
    minor: vmpage_io.memory.out

# convert from bytes to pages
- action: divide_int
  scale_factors_int:
    vmpage_io.swap.in: 4096
    vmpage_io.swap.out: 4096
    vmpage_io.memory.in: 4096
    vmpage_io.memory.out: 4096

# process metric
- action: copy_metrics
  mapping:
    process.cpu.time: sf_temp.process.cpu.time
  dimension_key: state
  dimension_values:
    user: true
    system: true

- action: aggregate_metric
  metric_name: sf_temp.process.cpu.time
  aggregation_method: sum
  without_dimensions:
  - state

- action: rename_metrics
  mapping:
    sf_temp.container_cpu_utilization: container_cpu_utilization
    sf_temp.cpu.idle: cpu.idle
    sf_temp.cpu.idle_per_core: cpu.idle
    sf_temp.cpu.interrupt: cpu.interrupt
    sf_temp.cpu.interrupt_per_core: cpu.interrupt
    sf_temp.cpu.nice: cpu.nice
    sf_temp.cpu.nice_per_core: cpu.nice
    sf_temp.cpu.num_processors: cpu.num_processors
    sf_temp.cpu.softirq: cpu.softirq
    sf_temp.cpu.softirq_per_core: cpu.softirq
    sf_temp.cpu.steal: cpu.steal
    sf_temp.cpu.steal_per_core: cpu.steal
    sf_temp.cpu.system: cpu.system
    sf_temp.cpu.system_per_core: cpu.system
    sf_temp.cpu.user: cpu.user
    sf_temp.cpu.user_per_core: cpu.user
    sf_temp.cpu.utilization: cpu.utilization
    sf_temp.cpu.wait: cpu.wait
    sf_temp.cpu.wait_per_core: cpu.wait
    sf_temp.disk.summary_utilization: disk.summary_utilization
    sf_temp.disk.utilization: disk.utilization
    sf_temp.memory.total: memory.total
    sf_temp.memory.utilization: memory.utilization
    sf_temp.network.total: network.total
    sf_temp.system.disk.io.total: system.disk.io.total
    sf_temp.system.disk.operations.total: system.disk.operations.total
    sf_temp.system.network.io.total: system.network.io.total
    sf_temp.system.network.packets.total: system.network.packets.total
    sf_temp.process.cpu.time: process.cpu_time_seconds

# remove redundant metrics
- action: drop_metrics
  metric_names:
    sf_temp.df_complex.used: true
    sf_temp.df_complex.used_total: true
    sf_temp.disk.ops: true
    sf_temp.disk.summary_total: true
    sf_temp.disk.total: true
    sf_temp.memory.used: true
    sf_temp.system.cpu.delta: true
    sf_temp.system.cpu.total: true
    sf_temp.system.cpu.time: true
    sf_temp.system.cpu.usage: true
    sf_temp.system.filesystem.usage: true
    sf_temp.system.memory.usage: true
    sf_temp.system.paging.operations: true
    sf_temp.system.paging.operations.page_in: true
    sf_temp.system.paging.operations.page_out: true
`
