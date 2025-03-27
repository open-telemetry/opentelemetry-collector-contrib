package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseUserAgent(useragent *modelpb.UserAgent, attrs pcommon.Map) {
	if useragent == nil {
		return
	}

	PutOptionalStr(attrs, conventions.AttributeHTTPUserAgent, &useragent.Original)
	PutOptionalStr(attrs, "useragent.name", &useragent.Name)
}
