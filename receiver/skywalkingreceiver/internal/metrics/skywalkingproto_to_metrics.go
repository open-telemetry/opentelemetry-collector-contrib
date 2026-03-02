// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver/internal/metrics"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	jvmScopeName            = "runtime_metrics"
	gcCountMetricName       = "sw.jvm.gc.count"
	gcDurationMetricName    = "sw.jvm.gc.duration"
	MemoryPoolInitName      = "jvm.memory.init"
	MemoryPoolMaxName       = "jvm.memory.max"
	MemoryPoolUsedName      = "jvm.memory.used"
	MemoryPoolCommittedName = "jvm.memory.committed"
	ThreadCountName         = "jvm.thread.count"
	CPUUtilizationName      = "jvm.cpu.recent_utilization"
)

func SwMetricsToMetrics(collection *agent.JVMMetricCollection) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, jvmMetric := range collection.GetMetrics() {
		resourceMetric := md.ResourceMetrics().AppendEmpty()
		scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()
		scopeMetric.Scope().SetName(jvmScopeName)
		jvmMetricToResourceMetrics(jvmMetric, scopeMetric)
		jvmMetricToResource(collection.Service, collection.ServiceInstance, resourceMetric.Resource())
	}
	return md
}

func jvmMetricToResource(serviceName, serviceInstance string, resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.EnsureCapacity(2)
	attrs.PutStr(string(conventions.ServiceNameKey), serviceName)
	attrs.PutStr(string(conventions.ServiceInstanceIDKey), serviceInstance)
}

func jvmMetricToResourceMetrics(jvmMetric *agent.JVMMetric, sm pmetric.ScopeMetrics) {
	// gc metric to otlp metric
	gcMetricToMetrics(jvmMetric.Time, jvmMetric.Gc, sm)
	// memory pool metric to otlp metric
	memoryPoolMetricToMetrics(jvmMetric.Time, jvmMetric.MemoryPool, sm)
	// thread metric to otlp metric
	threadMetricToMetrics(jvmMetric.Time, jvmMetric.Thread, sm)
	// cpu metric to otpl metric
	cpuMetricToMetrics(jvmMetric.Time, jvmMetric.Cpu, sm)
}

// gcMetricToMetrics translate gc metrics
func gcMetricToMetrics(timestamp int64, gcList []*agent.GC, dest pmetric.ScopeMetrics) {
	// gc count and gc duration is not
	metricCount := dest.Metrics().AppendEmpty()
	metricCount.SetName(gcCountMetricName)
	metricCountDps := metricCount.SetEmptyGauge().DataPoints()

	metricDuration := dest.Metrics().AppendEmpty()
	metricDuration.SetName(gcDurationMetricName)
	metricDuration.SetUnit("ms")
	metricDurationDps := metricDuration.SetEmptyGauge().DataPoints()
	for _, gc := range gcList {
		attrs := buildGCAttrs(gc)
		fillNumberDataPointIntValue(timestamp, gc.Count, metricCountDps.AppendEmpty(), attrs)
		fillNumberDataPointIntValue(timestamp, gc.Time, metricDurationDps.AppendEmpty(), attrs)
	}
}

func buildGCAttrs(gc *agent.GC) pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("jvm.gc.name", gc.GetPhase().String())
	return attrs
}

// memoryPoolMetricToMetrics translate memoryPool metrics
func memoryPoolMetricToMetrics(timestamp int64, memoryPools []*agent.MemoryPool, sm pmetric.ScopeMetrics) {
	PoolNameArr := []string{MemoryPoolInitName, MemoryPoolUsedName, MemoryPoolMaxName, MemoryPoolCommittedName}
	dpsMp := make(map[string]pmetric.NumberDataPointSlice)
	for _, name := range PoolNameArr {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		metric.SetUnit("By")
		dpsMp[name] = metric.SetEmptyGauge().DataPoints()
	}
	for _, memoryPool := range memoryPools {
		attrs := buildMemoryPoolAttrs(memoryPool)
		fillNumberDataPointIntValue(timestamp, memoryPool.Init, dpsMp[MemoryPoolInitName].AppendEmpty(), attrs)
		fillNumberDataPointIntValue(timestamp, memoryPool.Max, dpsMp[MemoryPoolMaxName].AppendEmpty(), attrs)
		fillNumberDataPointIntValue(timestamp, memoryPool.Used, dpsMp[MemoryPoolUsedName].AppendEmpty(), attrs)
		fillNumberDataPointIntValue(timestamp, memoryPool.Committed, dpsMp[MemoryPoolCommittedName].AppendEmpty(), attrs)
	}
}

func buildMemoryPoolAttrs(pool *agent.MemoryPool) pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("jvm.memory.pool.name", pool.GetType().String())

	var memoryType string
	switch pool.GetType() {
	case agent.PoolType_CODE_CACHE_USAGE, agent.PoolType_METASPACE_USAGE, agent.PoolType_PERMGEN_USAGE:
		memoryType = "non_heap"
	case agent.PoolType_NEWGEN_USAGE, agent.PoolType_OLDGEN_USAGE, agent.PoolType_SURVIVOR_USAGE:
		memoryType = "heap"
	default:
	}
	attrs.PutStr("jvm.memory.type", memoryType)
	return attrs
}

func threadMetricToMetrics(timestamp int64, thread *agent.Thread, dest pmetric.ScopeMetrics) {
	metric := dest.Metrics().AppendEmpty()
	metric.SetName(ThreadCountName)
	metric.SetUnit("{thread}")
	metricDps := metric.SetEmptyGauge().DataPoints()
	fillNumberDataPointIntValue(timestamp, thread.LiveCount, metricDps.AppendEmpty(), buildThreadTypeAttrs("live"))
	fillNumberDataPointIntValue(timestamp, thread.DaemonCount, metricDps.AppendEmpty(), buildThreadTypeAttrs("daemon"))
	fillNumberDataPointIntValue(timestamp, thread.PeakCount, metricDps.AppendEmpty(), buildThreadTypeAttrs("peak"))
	fillNumberDataPointIntValue(timestamp, thread.RunnableStateThreadCount, metricDps.AppendEmpty(), buildThreadTypeAttrs("runnable"))
	fillNumberDataPointIntValue(timestamp, thread.BlockedStateThreadCount, metricDps.AppendEmpty(), buildThreadTypeAttrs("blocked"))
	fillNumberDataPointIntValue(timestamp, thread.WaitingStateThreadCount, metricDps.AppendEmpty(), buildThreadTypeAttrs("waiting"))
	fillNumberDataPointIntValue(timestamp, thread.TimedWaitingStateThreadCount, metricDps.AppendEmpty(), buildThreadTypeAttrs("time_waiting"))
}

func buildThreadTypeAttrs(typeValue string) pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("thread.state", typeValue)
	var isDaemon bool
	if typeValue == "daemon" {
		isDaemon = true
	}
	attrs.PutBool("thread.daemon", isDaemon)
	return attrs
}

func cpuMetricToMetrics(timestamp int64, cpu *common.CPU, dest pmetric.ScopeMetrics) {
	metric := dest.Metrics().AppendEmpty()
	metric.SetName(CPUUtilizationName)
	metric.SetUnit("1")
	metricDps := metric.SetEmptyGauge().DataPoints()
	fillNumberDataPointDoubleValue(timestamp, cpu.UsagePercent, metricDps.AppendEmpty(), pcommon.NewMap())
}

func fillNumberDataPointIntValue(timestamp, value int64, point pmetric.NumberDataPoint, attrs pcommon.Map) {
	attrs.CopyTo(point.Attributes())
	point.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(timestamp)))
	point.SetIntValue(value)
}

func fillNumberDataPointDoubleValue(timestamp int64, value float64, point pmetric.NumberDataPoint, attrs pcommon.Map) {
	attrs.CopyTo(point.Attributes())
	point.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(timestamp)))
	point.SetDoubleValue(value)
}
