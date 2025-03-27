package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseAgent(agent *modelpb.Agent, attrs pcommon.Map) {
	if agent == nil {
		return
	}

	PutOptionalStr(attrs, conventions.AttributeTelemetrySDKName, &agent.Name)
	PutOptionalStr(attrs, conventions.AttributeTelemetrySDKVersion, &agent.Version)
	PutOptionalStr(attrs, "telemetry.sdk.ephemeral_id", &agent.EphemeralId)
	PutOptionalStr(attrs, "telemetry.sdk.activation_method", &agent.ActivationMethod)
}
