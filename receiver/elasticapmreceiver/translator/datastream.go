package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseDataStream(datastream *modelpb.DataStream, attrs pcommon.Map) {
	if datastream == nil {
		return
	}

	PutOptionalStr(attrs, "datastream.type", &datastream.Type)
	PutOptionalStr(attrs, "datastream.dataset", &datastream.Dataset)
	PutOptionalStr(attrs, "datastream.namespace", &datastream.Namespace)
}
