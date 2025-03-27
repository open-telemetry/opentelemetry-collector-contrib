package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseDevice(device *modelpb.Device, attrs pcommon.Map) {
	if device == nil {
		return
	}

	PutOptionalStr(attrs, conventions.AttributeDeviceID, &device.Id)
	parseDeviceModel(device.Model, attrs)
	PutOptionalStr(attrs, conventions.AttributeDeviceManufacturer, &device.Manufacturer)
}

func parseDeviceModel(devicemodel *modelpb.DeviceModel, attrs pcommon.Map) {
	if devicemodel == nil {
		return
	}

	PutOptionalStr(attrs, conventions.AttributeDeviceModelIdentifier, &devicemodel.Identifier)
	PutOptionalStr(attrs, conventions.AttributeDeviceModelName, &devicemodel.Name)
}
