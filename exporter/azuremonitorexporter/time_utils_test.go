// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToTime(t *testing.T) {
	// 61 seconds after the Unix epoch of 1970-01-01T00:00:00Z
	input := pcommon.Timestamp(60000000001)
	output := toTime(input)

	assert.NotNil(t, output)
	expected := time.Date(1970, 01, 01, 00, 01, 00, 1, time.UTC)
	assert.Equal(t, "1970-01-01T00:01:00.000000001Z", expected.Format(time.RFC3339Nano))
}

func TestFormatDuration(t *testing.T) {
	// 1 minute 30 second duration
	dur := time.Minute * 1
	dur += time.Second * 30

	output := formatDuration(dur)

	// DD.HH:MM:SS.MMMMMM
	assert.NotNil(t, output)
	assert.Equal(t, "00.00:01:30.000000", output)
}
