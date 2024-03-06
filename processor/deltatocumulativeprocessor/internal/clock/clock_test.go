// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFakeAfter(t *testing.T) {
	fake := Fake()

	ch := fake.After(10 * time.Second)
	now := fake.Now()

	fake.Set(now.Add(10 * time.Second))
	done := <-ch

	require.Equal(t, 10*time.Second, done.Sub(now))
}

func TestFakeSet(t *testing.T) {
	fake := Fake()
	require.Equal(t, time.Time{}, fake.Now())

	ts := time.Time{}.Add(10 * time.Minute)
	fake.Set(ts)
	require.Equal(t, ts, fake.Now())
}
