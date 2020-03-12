package redisreceiver

import "time"

type scheduler struct {
	intervalSeconds time.Duration
	tickables       []tickable
}

func newScheduler(intervalSeconds time.Duration, tickables []tickable) *scheduler {
	return &scheduler{
		intervalSeconds: intervalSeconds,
		tickables:       tickables,
	}
}

func (s *scheduler) start() error {
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

func (s *scheduler) setup() error {
	for _, tickable := range s.tickables {
		err := tickable.setup()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *scheduler) run() error {
	duration := s.intervalSeconds * time.Second
	ticker := time.NewTicker(duration)
	for range ticker.C {
		for _, tickable := range s.tickables {
			err := tickable.run()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *scheduler) stop() error {
	return nil
}
