//go:build windows

package perflib // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters/third_party/perflib"

import (
	"strconv"
)

func MapCounterToIndex(name string) string {
	return strconv.Itoa(int(CounterNameTable.LookupIndex(name)))
}
