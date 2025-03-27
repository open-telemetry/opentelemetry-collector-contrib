package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseDestination(destination *modelpb.Destination, attrs pcommon.Map) {
	if destination == nil {
		return
	}

	PutOptionalStr(attrs, "destionation.address", &destination.Address)
	PutOptionalInt(attrs, "destination.port", &destination.Port)
}
