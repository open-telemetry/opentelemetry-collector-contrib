package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseSession(session *modelpb.Session, attrs pcommon.Map) {
	if session == nil {
		return
	}

	PutOptionalStr(attrs, "session.id", &session.Id)
	PutOptionalInt(attrs, "session.sequence", &session.Sequence)
}
