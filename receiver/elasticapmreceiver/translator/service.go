package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseService(service *modelpb.Service, attrs pcommon.Map) {
	if service == nil {
		return
	}
	PutOptionalStr(attrs, conventions.AttributeServiceName, &service.Name)
	PutOptionalStr(attrs, conventions.AttributeServiceVersion, &service.Version)
	PutOptionalStr(attrs, conventions.AttributeDeploymentEnvironment, &service.Environment)
	parseServiceOrigin(service.Origin, attrs)
	parseServiceTarget(service.Target, attrs)
	parseLanguage(service.Language, attrs)
	parseRuntime(service.Runtime, attrs)
	parseFramework(service.Framework, attrs)
	parseServiceNode(service.Node, attrs)
}

func parseServiceOrigin(origin *modelpb.ServiceOrigin, attrs pcommon.Map) {
	if origin == nil {
		return
	}
	PutOptionalStr(attrs, "service.origin.id", &origin.Id)
	PutOptionalStr(attrs, "service.origin.name", &origin.Name)
	PutOptionalStr(attrs, "service.origin.version", &origin.Version)
}

func parseServiceTarget(target *modelpb.ServiceTarget, attrs pcommon.Map) {
	if target == nil {
		return
	}
	PutOptionalStr(attrs, "service.target.name", &target.Name)
	PutOptionalStr(attrs, "service.target.type", &target.Type)
}

func parseLanguage(language *modelpb.Language, attrs pcommon.Map) {
	if language == nil {
		return
	}
	PutOptionalStr(attrs, conventions.AttributeTelemetrySDKLanguage, &language.Name)
	PutOptionalStr(attrs, "telemetry.sdk.language_version", &language.Version)
}

func parseRuntime(runtime *modelpb.Runtime, attrs pcommon.Map) {
	if runtime == nil {
		return
	}
	PutOptionalStr(attrs, conventions.AttributeProcessRuntimeName, &runtime.Name)
	PutOptionalStr(attrs, conventions.AttributeProcessRuntimeVersion, &runtime.Version)
}

func parseFramework(framework *modelpb.Framework, attrs pcommon.Map) {
	if framework == nil {
		return
	}
	PutOptionalStr(attrs, "service.framework.name", &framework.Name)
	PutOptionalStr(attrs, "service.framework.version", &framework.Version)
}

func parseServiceNode(node *modelpb.ServiceNode, attrs pcommon.Map) {
	if node == nil {
		return
	}
	PutOptionalStr(attrs, "service.node.name", &node.Name)
}
