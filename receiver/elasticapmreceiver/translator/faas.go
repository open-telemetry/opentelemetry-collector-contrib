package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseFaas(faas *modelpb.Faas, attrs pcommon.Map) {
	if faas == nil {
		return
	}

	PutOptionalStr(attrs, conventions.AttributeFaaSID, &faas.Id)
	PutOptionalBool(attrs, conventions.AttributeFaaSColdstart, faas.ColdStart)
	PutOptionalStr(attrs, conventions.AttributeFaaSExecution, &faas.Execution)
	PutOptionalStr(attrs, conventions.AttributeFaaSTrigger, &faas.TriggerType)
	PutOptionalStr(attrs, "faas.trigger.request_id", &faas.TriggerRequestId)
	PutOptionalStr(attrs, conventions.AttributeFaaSName, &faas.Name)
	PutOptionalStr(attrs, conventions.AttributeFaaSVersion, &faas.Version)
}
