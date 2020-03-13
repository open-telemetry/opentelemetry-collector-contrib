package redisreceiver

import "time"

type intervalRunner struct {
	intervalRunnables []intervalRunnable
	ticker            *time.Ticker
}

func newIntervalRunner(interval time.Duration, intervalRunnables ...intervalRunnable) *intervalRunner {
	return &intervalRunner{
		intervalRunnables: intervalRunnables,
		ticker:            time.NewTicker(interval),
	}
}

func (s *intervalRunner) start() error {
	err := s.setup()
	if err != nil {
		return err
	}
	err = s.run()
	if err != nil {
		return err
	}
	return nil
}

func (s *intervalRunner) setup() error {
	for _, r := range s.intervalRunnables {
		err := r.setup()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *intervalRunner) run() error {
	for range s.ticker.C {
		for _, runnable := range s.intervalRunnables {
			err := runnable.run()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *intervalRunner) stop() {
	s.ticker.Stop()
}
