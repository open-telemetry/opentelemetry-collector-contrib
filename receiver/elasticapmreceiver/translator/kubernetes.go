package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseKubernetes(kubernetes *modelpb.Kubernetes, attrs pcommon.Map) {
	if kubernetes == nil {
		return
	}
	PutOptionalStr(attrs, conventions.AttributeK8SNamespaceName, &kubernetes.Namespace)
	PutOptionalStr(attrs, conventions.AttributeK8SNodeName, &kubernetes.NodeName)
	PutOptionalStr(attrs, conventions.AttributeK8SPodName, &kubernetes.PodName)
	PutOptionalStr(attrs, conventions.AttributeK8SPodUID, &kubernetes.PodUid)
}
