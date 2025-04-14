// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"time"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (c *realClock) Now() time.Time {
	return time.Now()
}

type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time {
	c.now = c.now.Add(100 * time.Millisecond) // Add 100ms to the mock clock to avoid timestamp collisions
	return c.now
}
