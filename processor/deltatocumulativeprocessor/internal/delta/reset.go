// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"

import "go.opentelemetry.io/collector/pdata/pcommon"

type ZeroType[Self any] interface {
	Type[Self]

	Attributes() pcommon.Map
	SetStartTimestamp(pcommon.Timestamp)
}

// GetZeroDp creates a zero-valued datapoint at startTimestamp-1ns.
func GetZeroDp[T ZeroType[T]](dp T, newDp func() T, setZero func(src, dst T)) T {
	zeroDp := newDp()
	dp.Attributes().CopyTo(zeroDp.Attributes())
	ts := pcommon.Timestamp(uint64(dp.StartTimestamp()) - 1)
	zeroDp.SetStartTimestamp(ts)
	zeroDp.SetTimestamp(ts)
	setZero(dp, zeroDp)
	return zeroDp
}
