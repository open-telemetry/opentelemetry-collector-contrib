// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeutils

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testObj struct {
	mu  sync.Mutex
	foo int
}

func (to *testObj) IncrementFoo() {
	to.mu.Lock()
	defer to.mu.Unlock()
	to.foo++
}

func checkFooValue(to *testObj, expectedValue int) bool {
	to.mu.Lock()
	defer to.mu.Unlock()
	return to.foo == expectedValue
}

func TestPolicyTickerFails(t *testing.T) {
	to := &testObj{foo: 0}
	pTicker := &PolicyTicker{OnTickFunc: to.IncrementFoo}

	// Tickers with a duration <= 0 should panic
	assert.Panics(t, func() { pTicker.Start(time.Duration(0)) })
	assert.Panics(t, func() { pTicker.Start(time.Duration(-1)) })
}

func TestPolicyTickerStart(t *testing.T) {
	to := &testObj{foo: 0}
	pTicker := &PolicyTicker{OnTickFunc: to.IncrementFoo}

	// Make sure no ticks occur when we immediately stop the ticker
	time.Sleep(100 * time.Millisecond)
	assert.True(t, checkFooValue(to, 0), "Expected value: %d, actual: %d", 0, to.foo)
	pTicker.Start(1 * time.Second)
	pTicker.Stop()
	assert.True(t, checkFooValue(to, 0), "Expected value: %d, actual: %d", 0, to.foo)
}

func TestPolicyTickerSucceeds(t *testing.T) {
	// Start the ticker, make sure variable is incremented properly,
	// also make sure stop works as expected.
	to := &testObj{foo: 0}
	pTicker := &PolicyTicker{OnTickFunc: to.IncrementFoo}

	expectedTicks := 4
	defaultDuration := 500 * time.Millisecond
	// Extra padding reduces the chance of a race happening here
	padDuration := 200 * time.Millisecond
	testSleepDuration := time.Duration(expectedTicks)*defaultDuration + padDuration

	pTicker.Start(defaultDuration)
	time.Sleep(testSleepDuration)
	assert.True(t, checkFooValue(to, expectedTicks), "Expected value: %d, actual: %d", expectedTicks, to.foo)

	pTicker.Stop()
	// Since these tests are time sensitive they can be flaky. By getting the count
	// after stopping, we can still test to make sure it's no longer being incremented,
	// without requiring there by no OnTick calls between the sleep call and stopping.
	time.Sleep(defaultDuration)
	expectedTicks = to.foo

	time.Sleep(testSleepDuration)
	assert.True(t, checkFooValue(to, expectedTicks), "Expected value: %d, actual: %d", expectedTicks, to.foo)
}
