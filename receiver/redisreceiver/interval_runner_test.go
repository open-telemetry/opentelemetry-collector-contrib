package redisreceiver

import (
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	f := &fakeRunnable{}
	s := newIntervalRunner(time.Second, f)
	go func() {
		_ = s.start()
	}()
	s.stop()
	// getting here is success
}

type fakeRunnable struct{
}

func (t *fakeRunnable) setup() error {
	return nil
}

func (fakeRunnable) run() error {
	return nil
}
