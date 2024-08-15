// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
)

// AccumulateInto adds state and dp, storing the result in state
//
//	state = state + dp
func AccumulateInto[P data.Point[P]](state P, dp P) error {
	switch {
	case dp.StartTimestamp() < state.StartTimestamp():
		// belongs to older series
		return ErrOlderStart{Start: state.StartTimestamp(), Sample: dp.StartTimestamp()}
	case dp.Timestamp() <= state.Timestamp():
		// out of order
		return ErrOutOfOrder{Last: state.Timestamp(), Sample: dp.Timestamp()}
	}

	state.Add(dp)
	return nil
}

type ErrOlderStart struct {
	Start  pcommon.Timestamp
	Sample pcommon.Timestamp
}

func (e ErrOlderStart) Error() string {
	return fmt.Sprintf("dropped sample with start_time=%s, because series only starts at start_time=%s. consider checking for multiple processes sending the exact same series", e.Sample, e.Start)
}

type ErrOutOfOrder struct {
	Last   pcommon.Timestamp
	Sample pcommon.Timestamp
}

func (e ErrOutOfOrder) Error() string {
	return fmt.Sprintf("out of order: dropped sample from time=%s, because series is already at time=%s", e.Sample, e.Last)
}
