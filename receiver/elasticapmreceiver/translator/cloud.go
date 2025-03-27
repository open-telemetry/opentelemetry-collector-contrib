package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseCloud(cloud *modelpb.Cloud, attrs pcommon.Map) {
	if cloud == nil {
		return
	}
	// Origin           *CloudOrigin `protobuf:"bytes,1,opt,name=origin,proto3" json:"origin,omitempty"`
	PutOptionalStr(attrs, conventions.AttributeCloudAccountID, &cloud.AccountId)
	PutOptionalStr(attrs, "cloud.account.name", &cloud.AccountName)
	PutOptionalStr(attrs, conventions.AttributeCloudAvailabilityZone, &cloud.AvailabilityZone)
	PutOptionalStr(attrs, "cloud.platform.id", &cloud.InstanceId)
	PutOptionalStr(attrs, conventions.AttributeCloudPlatform, &cloud.InstanceName)
	PutOptionalStr(attrs, "cloud.machine.type", &cloud.MachineType)
	PutOptionalStr(attrs, "cloud.project.id", &cloud.ProjectId)
	PutOptionalStr(attrs, "cloud.project.name", &cloud.ProjectName)
	PutOptionalStr(attrs, conventions.AttributeCloudProvider, &cloud.Provider)
	PutOptionalStr(attrs, conventions.AttributeCloudRegion, &cloud.Region)
	PutOptionalStr(attrs, "cloud.service.name", &cloud.ServiceName)
}
