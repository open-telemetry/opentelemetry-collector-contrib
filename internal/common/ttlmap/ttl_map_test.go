// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ttlmap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTTLMapData(t *testing.T) {
	m := newTTLMapData(10)
	require.Nil(t, m.get("foo"))
	m.put("bob", "xyz", 2)
	m.sweep(12)
	require.NotNil(t, m.get("bob"))
	m.sweep(13)
	require.Nil(t, m.get("bob"))
}

func TestTTLMapSimple(t *testing.T) {
	m := New(5, 10, make(chan struct{}))
	require.EqualValues(t, 5, m.sweepInterval)
	require.EqualValues(t, 10, m.md.maxAge)
	m.Put("foo", "bar")
	s := m.Get("foo").(string)
	require.Equal(t, "bar", s)
}

func TestTTLMapLong(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TestTTLMapLong in short mode")
	}
	m := New(1, 1, make(chan struct{}))
	m.Start()
	defer m.Shutdown()
	m.Put("foo", "bar")
	require.Equal(t, "bar", m.Get("foo"))
	time.Sleep(time.Second * 3)
	require.Nil(t, m.Get("foo"))
}
