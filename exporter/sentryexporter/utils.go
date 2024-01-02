// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// unixNanoToTime converts UNIX Epoch time in nanoseconds
// to a Time struct.
func unixNanoToTime(u pcommon.Timestamp) time.Time {
	return time.Unix(0, int64(u)).UTC()
}
