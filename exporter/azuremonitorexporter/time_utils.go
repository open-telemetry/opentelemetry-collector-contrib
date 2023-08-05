// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

// Contains code common to both trace and metrics exporters

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func toTime(t pcommon.Timestamp) time.Time {
	return time.Unix(0, int64(t))
}

// Formats a Duration into the form DD.HH:MM:SS.MMMMMM
func formatDuration(d time.Duration) string {
	day := d / (time.Hour * 24)
	d -= day * (time.Hour * 24) // nolint: durationcheck

	h := d / time.Hour
	d -= h * time.Hour // nolint: durationcheck

	m := d / time.Minute
	d -= m * time.Minute // nolint: durationcheck

	s := d / time.Second
	d -= s * time.Second // nolint: durationcheck

	us := d / time.Microsecond

	return fmt.Sprintf("%02d.%02d:%02d:%02d.%06d", day, h, m, s, us)
}

var timeNow = time.Now
