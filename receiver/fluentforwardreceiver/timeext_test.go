// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeEventExp(t *testing.T) {
	tim := time.Unix(500, 250)
	e := eventTimeExt(tim)

	require.Equal(t, 8, e.Len())
	var b [8]byte

	err := e.MarshalBinaryTo(b[:])
	require.Nil(t, err)
	require.Equal(t, []byte{0x00, 0x00, 0x01, 0xf4, 0x00, 0x00, 0x00, 0xfa}, b[:])

	err = e.UnmarshalBinary(b[:])
	require.Nil(t, err)
	require.Equal(t, tim, time.Time(e))

	err = e.UnmarshalBinary(b[:5])
	require.NotNil(t, err)
}
