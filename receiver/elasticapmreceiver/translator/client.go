package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseClient(client *modelpb.Client, attrs pcommon.Map) {
	if client == nil {
		return
	}

	parseClientIP(client.Ip, attrs)
	PutOptionalStr(attrs, "client.domain", &client.Domain)
	PutOptionalInt(attrs, "client.port", &client.Port)
}

func parseClientIP(ip *modelpb.IP, attrs pcommon.Map) {
	if ip == nil {
		return
	}

	if &ip.V4 != nil {
		attrs.PutStr("client.ip.v4", parseIPV4(ip.V4))
	}

	if &ip.V6 != nil && len(ip.V6) == 16 {
		attrs.PutStr("client.ip.v6", parseIPV6(ip.V6))
	}
}
