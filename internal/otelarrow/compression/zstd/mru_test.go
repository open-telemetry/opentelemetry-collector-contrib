// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zstd

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type gint struct {
	value int
	Gen
}

func TestMRUGet(t *testing.T) {
	defer resetTest()

	var m mru[*gint]
	const cnt = 5

	v, g := m.Get()
	require.Nil(t, v)

	for i := 0; i < cnt; i++ {
		p := &gint{
			value: i + 1,
			Gen:   g,
		}
		m.Put(p)
	}

	for i := 0; i < cnt; i++ {
		v, _ = m.Get()
		require.Equal(t, 5-i, v.value)
	}

	v, _ = m.Get()
	require.Nil(t, v)
}

func TestMRUPut(t *testing.T) {
	defer resetTest()

	var m mru[*gint]
	const cnt = 5

	// Use zero TTL => no freelist
	TTL = 0

	g := m.Reset()

	for i := 0; i < cnt; i++ {
		p := &gint{
			value: i + 1,
			Gen:   g,
		}
		m.Put(p)
	}
	require.Equal(t, 0, m.Size())
}

func TestMRUReset(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34252")
	}

	defer resetTest()

	var m mru[*gint]

	g := m.Reset()

	m.Put(&gint{
		Gen: g,
	})
	require.Equal(t, 1, m.Size())

	// Ensure the monotonic clock has has advanced before resetting.
	time.Sleep(10 * time.Millisecond)

	m.Reset()
	require.Equal(t, 0, m.Size())

	// This doesn't take because its generation is before the reset.
	m.Put(&gint{
		Gen: g,
	})
	require.Equal(t, 0, m.Size())
}
