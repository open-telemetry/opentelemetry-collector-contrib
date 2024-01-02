// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func addStartTime(startTime *float64, span ptrace.Span) {
	span.SetStartTimestamp(floatSecToNanoEpoch(startTime))
}

func addEndTime(endTime *float64, span ptrace.Span) {
	if endTime != nil {
		span.SetEndTimestamp(floatSecToNanoEpoch(endTime))
	}
}

func floatSecToNanoEpoch(epochSec *float64) pcommon.Timestamp {
	return pcommon.Timestamp((*epochSec) * float64(time.Second))
}
