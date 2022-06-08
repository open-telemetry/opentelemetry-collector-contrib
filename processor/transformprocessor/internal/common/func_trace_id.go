package common

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func traceID(bytes []byte) (ExprFunc, error) {
	if len(bytes) != 16 {
		return nil, errors.New("traces ids must be 16 bytes")
	}
	var idArr [16]byte
	copy(idArr[:16], bytes)
	traceId := pcommon.NewTraceID(idArr)
	return func(ctx TransformContext) interface{} {
		return traceId
	}, nil
}
