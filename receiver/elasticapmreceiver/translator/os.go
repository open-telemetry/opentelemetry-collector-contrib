package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseOS(os *modelpb.OS, attrs pcommon.Map) {
	if os == nil {
		return
	}

	PutOptionalStr(attrs, conventions.AttributeOSName, &os.Name)
	PutOptionalStr(attrs, conventions.AttributeOSVersion, &os.Version)
	PutOptionalStr(attrs, "os.platform", &os.Platform)
	PutOptionalStr(attrs, conventions.AttributeOSDescription, &os.Full)
	PutOptionalStr(attrs, conventions.AttributeOSType, &os.Type)
}
