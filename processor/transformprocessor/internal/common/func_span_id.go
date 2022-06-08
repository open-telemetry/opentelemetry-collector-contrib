package common

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func spanID(bytes []byte) (ExprFunc, error) {
	if len(bytes) != 8 {
		return nil, errors.New("span ids must be 8 bytes")
	}
	var idArr [8]byte
	copy(idArr[:8], bytes)
	spanId := pcommon.NewSpanID(idArr)
	return func(ctx TransformContext) interface{} {
		return spanId
	}, nil
}
