package internal

import (
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// CreateResource creates the resource data added to OTLP payloads.
func CreateResource(projectId string, model.Metrics ) pcommon.Resource {
}