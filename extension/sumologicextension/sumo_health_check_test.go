package sumologicextension

import (
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.uber.org/zap"
	"sync"
	"testing"
	"time"
)

func TestInitHealthReporting(t *testing.T) {
	// Setup
	logger := zap.NewNop()

	hc := &SumoHealthCheck{
		logger:               logger,
		statusSubscriptionWg: &sync.WaitGroup{},
		componentHealthWg:    &sync.WaitGroup{},
		closeChan:            make(chan struct{}),
	}

	hc.initHealthReporting()

	assert.NotNil(t, hc.statusAggregator, "statusAggregator should be initialized")
	assert.NotNil(t, hc.componentStatusCh, "componentStatusCh should be initialized")

	AssertWaitGroupActive(t, hc.statusSubscriptionWg, "statusSubscriptionWg")
	AssertWaitGroupActive(t, hc.componentHealthWg, "componentHealthWg")

	close(hc.closeChan)

	AssertWaitGroupDone(t, hc.statusSubscriptionWg, "statusSubscriptionWg")
	AssertWaitGroupDone(t, hc.componentHealthWg, "componentHealthWg")
}

func TestComponentHealthReport(t *testing.T) {
	// Setup
	logger := zap.NewNop()

	hc := &SumoHealthCheck{
		logger:               logger,
		statusSubscriptionWg: &sync.WaitGroup{},
		componentHealthWg:    &sync.WaitGroup{},
		closeChan:            make(chan struct{}),
	}

	hc.initHealthReporting()

	componentType := component.MustNewType("testType")
	componentId := component.NewID(componentType)
	instance := componentstatus.NewInstanceID(componentId, component.KindReceiver)

	// Create a sample Event
	event := componentstatus.NewEvent(componentstatus.StatusOK)

	pair := eventSourcePair{
		source: instance,
		event:  event,
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		select {
		case esp := <-hc.componentStatusCh:
			assert.Equal(t, event, esp.event)
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for componentStatusCh event")
		}
	}()
	hc.componentStatusCh <- &pair
	<-done
	close(hc.closeChan)

}

func AssertWaitGroupActive(t *testing.T, wg *sync.WaitGroup, label string) {
	t.Helper()

	assert.NotNil(t, wg, label+" should not be nil")

	wgChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgChan)
	}()

	select {
	case <-wgChan:
		t.Errorf("%s counter is zero, expected it to be active", label)
	case <-time.After(100 * time.Millisecond):
		t.Logf("%s initialized and active", label)
	}
}

func AssertWaitGroupDone(t *testing.T, wg *sync.WaitGroup, label string) {
	t.Helper()

	assert.NotNil(t, wg, label+" should not be nil")

	wgChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgChan)
	}()

	select {
	case <-wgChan:
		t.Logf("%s is done (counter is zero)", label)
	case <-time.After(100 * time.Millisecond):
		t.Errorf("%s is not done (counter > 0), expected it to be done", label)
	}
}
