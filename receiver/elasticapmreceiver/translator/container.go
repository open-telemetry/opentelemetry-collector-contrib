package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseContainer(container *modelpb.Container, attrs pcommon.Map) {
	if container == nil {
		return
	}

	PutOptionalStr(attrs, conventions.AttributeContainerID, &container.Id)
	PutOptionalStr(attrs, conventions.AttributeContainerName, &container.Name)
	PutOptionalStr(attrs, conventions.AttributeContainerRuntime, &container.Runtime)
	PutOptionalStr(attrs, conventions.AttributeContainerImageName, &container.ImageName)
	PutOptionalStr(attrs, conventions.AttributeContainerImageTag, &container.ImageTag)
}
