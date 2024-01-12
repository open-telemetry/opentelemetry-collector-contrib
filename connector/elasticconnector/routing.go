package elasticconnector

import 	"go.opentelemetry.io/collector/pdata/pcommon"

var scopeNameMappings = map[string]string {
	"otelcol/hostmetricsreceiver/process": "system.process",
}

func getDatasetForScope(scope pcommon.InstrumentationScope) string {
	val, _ := scopeNameMappings[scope.Name()]
	return val
} 