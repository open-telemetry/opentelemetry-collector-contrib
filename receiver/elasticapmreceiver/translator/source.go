package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseSource(source *modelpb.Source, attrs pcommon.Map) {
	if source == nil {
		return
	}

	// TODO
}
