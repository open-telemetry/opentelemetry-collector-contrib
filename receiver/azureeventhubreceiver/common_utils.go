package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	azureResourceID = "azure.resource.id"
	scopeName       = "otelcol/azureresourcemetrics"
)

// asTimestamp will parse an ISO8601 string into an OpenTelemetry
// nanosecond timestamp. If the string cannot be parsed, it will
// return zero and the error.
func asTimestamp(s string) (pcommon.Timestamp, error) {
	t, err := iso8601.ParseString(s)
	if err != nil {
		return 0, err
	}
	return pcommon.Timestamp(t.UnixNano()), nil
}
