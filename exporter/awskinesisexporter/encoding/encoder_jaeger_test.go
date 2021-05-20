package encoding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/encoding"
)

func TestEncodingTraceData(t *testing.T) {
	t.Parallel()

	assert.NoError(t, encoding.Jaeger(nil).EncodeTraces(pdata.NewTraces()), "Must not error when processing spans")
}

func TestEncodingMetricData(t *testing.T) {
	t.Parallel()

	assert.Error(t, encoding.Jaeger(nil).EncodeMetrics(pdata.NewMetrics()), "Must error when trying to encode unsupported type")
}

func TestEncodingLogData(t *testing.T) {
	t.Parallel()

	assert.Error(t, encoding.Jaeger(nil).EncodeLogs(pdata.NewLogs()), "Must error when trying to encode unsupported type")
}
