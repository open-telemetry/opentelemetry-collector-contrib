package encoding

import (
	"errors"

	"go.opentelemetry.io/collector/consumer/pdata"
)

var (
	ErrUnsupportedEncodedType = errors.New("unsupported type to encode")
)

// Encoder allows for the internal types to be converted to an consumable
// exported type which is written to the kinesis stream
type Encoder interface {
	EncodeMetrics(md pdata.Metrics) error

	EncodeTraces(td pdata.Traces) error

	EncodeLogs(ld pdata.Logs) error
}
