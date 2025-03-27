package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseObserver(observer *modelpb.Observer, attrs pcommon.Map) {
	if observer == nil {
		return
	}

	PutOptionalStr(attrs, "observer.hostname", &observer.Hostname)
	PutOptionalStr(attrs, "observer.name", &observer.Name)
	PutOptionalStr(attrs, "observer.type", &observer.Type)
	PutOptionalStr(attrs, "observer.version", &observer.Version)
}
