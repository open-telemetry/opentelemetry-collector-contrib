package attributes

import (
	"errors"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
)

var (
	SignalTypeLogs = attribute.NewSet(attribute.String("signal", "logs"))
)

func InsertAttrIfMissingOrShouldOverride(overrideIncoming bool, sattr pcommon.Map, key string, value any) (err error) {
	if _, ok := sattr.Get(key); overrideIncoming || !ok {
		switch v := value.(type) {
		case string:
			sattr.PutStr(key, v)
		case int64:
			sattr.PutInt(key, v)
		default:
			err = errors.New("unsupported value type")
		}
	}
	return err
}
