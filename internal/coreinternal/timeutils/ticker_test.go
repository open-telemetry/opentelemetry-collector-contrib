package timeutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testObj struct {
	foo int
}

func (to *testObj) IncrementFoo() {
	to.foo += 1
}

func TestPolicyTickerFails(t *testing.T) {
	to := testObj{foo: 0}
	pTicker := &PolicyTicker{OnTickFunc: to.IncrementFoo}

	// Tickers with a duration <= 0 should panic
	assert.Panics(t, func() { pTicker.Start(time.Duration(0)) })
	assert.Panics(t, func() { pTicker.Start(time.Duration(-1)) })
}

func TestPolicyTickerStart(t *testing.T) {
	to := testObj{foo: 0}
	pTicker := &PolicyTicker{OnTickFunc: to.IncrementFoo}

	// Make sure no ticks occur when we immediately stop the ticker
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, to.foo)
	pTicker.Start(1 * time.Second)
	pTicker.Stop()
	assert.Equal(t, 0, to.foo)
}

func TestPolicyTickerSucceeds(t *testing.T) {
	// Start the ticker, make sure variable is incremented properly,
	// also make sure stop works as expected.
	to := testObj{foo: 0}
	pTicker := &PolicyTicker{OnTickFunc: to.IncrementFoo}

	// Ticker is first called after required duration, not at start. This means
	// expected count will be 1 less than how many durations have passed.
	expectedTicks := 4
	defaultDuration := 200 * time.Millisecond
	testSleepDuration := time.Duration(expectedTicks+1) * defaultDuration

	pTicker.Start(defaultDuration)
	time.Sleep(testSleepDuration)
	assert.Equal(t, expectedTicks, to.foo)

	pTicker.Stop()
	// Since these tests are time sensitive they can be flaky. By getting the count
	// after stopping, we can still test to make sure it's no longer being incremented,
	// without requiring there by no OnTick calls between the sleep call and stopping.
	expectedTicks = to.foo
	time.Sleep(testSleepDuration)
	assert.Equal(t, expectedTicks, to.foo)
}
