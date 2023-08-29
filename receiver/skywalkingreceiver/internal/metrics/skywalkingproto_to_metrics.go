package metrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	"time"
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
	CPUUtilizationName      = "jvm.cpu.utilization"
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

func jvmMetricToResource(serviceName string, serviceInstance string, resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.EnsureCapacity(2)
	attrs.PutStr(semconv.AttributeServiceName, serviceName)
	attrs.PutStr(semconv.AttributeServiceInstanceID, serviceInstance)
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
	metric := dest.Metrics().AppendEmpty()
	metric.SetName(gcCountMetricName)
	metricDps := metric.SetEmptyGauge().DataPoints()

	metricGcDuration := dest.Metrics().AppendEmpty()
	metricGcDuration.SetName(gcDurationMetricName)
	metricGcDuration.SetUnit("s")
	metricGcDurationDps := metricGcDuration.SetEmptyGauge().DataPoints()
	for _, gc := range gcList {
		attrs := buildGCAttrs(gc)
		fillNumberDataPoint(timestamp, float64(gc.Count), metricDps.AppendEmpty(), attrs)
		fillNumberDataPoint(timestamp, float64(gc.Time), metricGcDurationDps.AppendEmpty(), attrs)
	}
}

func buildGCAttrs(gc *agent.GC) pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("jvm.gc.name", gc.GetPhase().String())
	return attrs
}

// memoryPoolMetricToMetrics  translate memoryPool metrics
func memoryPoolMetricToMetrics(timestamp int64, memoryPools []*agent.MemoryPool, sm pmetric.ScopeMetrics) {
	nameArr := []string{MemoryPoolInitName, MemoryPoolUsedName, MemoryPoolMaxName, MemoryPoolCommittedName}
	dpsMp := make(map[string]pmetric.NumberDataPointSlice)
	for _, name := range nameArr {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		metric.SetUnit("By")
		dpsMp[name] = metric.SetEmptyGauge().DataPoints()
	}
	for _, memoryPool := range memoryPools {
		attrs := buildMemoryPoolAttrs(memoryPool)
		fillNumberDataPoint(timestamp, float64(memoryPool.Init), dpsMp[MemoryPoolInitName].AppendEmpty(), attrs)
		fillNumberDataPoint(timestamp, float64(memoryPool.Max), dpsMp[MemoryPoolMaxName].AppendEmpty(), attrs)
		fillNumberDataPoint(timestamp, float64(memoryPool.Used), dpsMp[MemoryPoolUsedName].AppendEmpty(), attrs)
		fillNumberDataPoint(timestamp, float64(memoryPool.Committed), dpsMp[MemoryPoolCommittedName].AppendEmpty(), attrs)
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
	fillNumberDataPoint(timestamp, float64(thread.LiveCount), metricDps.AppendEmpty(), buildThreadTypeAttrs("live"))
	fillNumberDataPoint(timestamp, float64(thread.DaemonCount), metricDps.AppendEmpty(), buildThreadTypeAttrs("daemon"))
	fillNumberDataPoint(timestamp, float64(thread.PeakCount), metricDps.AppendEmpty(), buildThreadTypeAttrs("peak"))
	fillNumberDataPoint(timestamp, float64(thread.RunnableStateThreadCount), metricDps.AppendEmpty(), buildThreadTypeAttrs("runnable"))
	fillNumberDataPoint(timestamp, float64(thread.BlockedStateThreadCount), metricDps.AppendEmpty(), buildThreadTypeAttrs("blocked"))
	fillNumberDataPoint(timestamp, float64(thread.WaitingStateThreadCount), metricDps.AppendEmpty(), buildThreadTypeAttrs("waiting"))
	fillNumberDataPoint(timestamp, float64(thread.TimedWaitingStateThreadCount), metricDps.AppendEmpty(), buildThreadTypeAttrs("time_waiting"))
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
	metricDps := metric.SetEmptyGauge().DataPoints()
	fillNumberDataPoint(timestamp, cpu.UsagePercent, metricDps.AppendEmpty(), pcommon.NewMap())
}

func fillNumberDataPoint(dateTime int64, value float64, point pmetric.NumberDataPoint, attrs pcommon.Map) {
	attrs.CopyTo(point.Attributes())
	point.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(dateTime)))
	point.SetDoubleValue(value)
}
