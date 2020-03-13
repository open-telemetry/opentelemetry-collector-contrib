package redisreceiver

import (
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	f := &fakeTickable{}
	s := newIntervalRunner(time.Second, f)
	go func() {
		_ = s.start()
	}()
	s.stop()
	// getting here is success
}

type fakeTickable struct{
}

func (t *fakeTickable) setup() error {
	return nil
}

func (fakeTickable) run() error {
	return nil
}
