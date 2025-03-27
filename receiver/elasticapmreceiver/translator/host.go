package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseHost(host *modelpb.Host, attrs pcommon.Map) {
	if host == nil {
		return
	}

	parseOS(host.Os, attrs)
	PutOptionalStr(attrs, conventions.AttributeNetHostName, &host.Hostname)
	PutOptionalStr(attrs, conventions.AttributeHostName, &host.Name)
	PutOptionalStr(attrs, conventions.AttributeHostID, &host.Id)
	PutOptionalStr(attrs, conventions.AttributeHostArch, &host.Architecture)
	PutOptionalStr(attrs, conventions.AttributeHostType, &host.Type)

	for _, ip := range host.Ip {
		if &ip.V4 != nil {
			attrs.PutStr("host.ip.v4", parseIPV4(ip.V4))
		}
		if &ip.V6 != nil && len(ip.V6) == 16 {
			attrs.PutStr("host.ip.v6", parseIPV6(ip.V6))
		}
	}
}
