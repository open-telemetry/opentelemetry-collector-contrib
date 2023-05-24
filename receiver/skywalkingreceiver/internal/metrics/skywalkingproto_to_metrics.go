package metrics

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func SkywalingToMetrics(collection *agent.JVMMetricCollection) pmetric.Metrics {

	return pmetric.NewMetrics()
}
