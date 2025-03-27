package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseUser(user *modelpb.User, attrs pcommon.Map) {
	if user == nil {
		return
	}

	PutOptionalStr(attrs, "user.domain", &user.Domain)
	PutOptionalStr(attrs, "user.id", &user.Id)
	PutOptionalStr(attrs, "user.email", &user.Email)
	PutOptionalStr(attrs, "user.name", &user.Name)
}
