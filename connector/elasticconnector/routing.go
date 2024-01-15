package elasticconnector

import "go.opentelemetry.io/collector/pdata/pcommon"

var scopeNameMappings = map[string]string{
	"otelcol/hostmetricsreceiver/cpu":  "system.cpu",
	"otelcol/hostmetricsreceiver/disk": "system.diskio",
	"otelcol/hostmetricsreceiver/load": "system.load",
	//	"otelcol/hostmetricsreceiver/filesystem": "",
	"otelcol/hostmetricsreceiver/memory":    "system.memory",
	"otelcol/hostmetricsreceiver/network":   "system.network",
	"otelcol/hostmetricsreceiver/paging":    "system.memory",
	"otelcol/hostmetricsreceiver/processes": "system.process",
	"otelcol/hostmetricsreceiver/process":   "system.process",
}

func getDatasetForScope(scope pcommon.InstrumentationScope) string {
	val, _ := scopeNameMappings[scope.Name()]
	return val
}
