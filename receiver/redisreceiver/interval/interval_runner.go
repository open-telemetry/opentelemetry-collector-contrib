package interval

import "time"

// Runner takes a list of `intervalRunnable`s,
// calls setup() on all of them and then calls run() on
// all of them. It currently does so sequentially and
// within the same goroutine.
type Runner struct {
	runnables []Runnable
	ticker    *time.Ticker
}

// Creates a new Runner. Pass in a duration (time between calls)
// and one or more runnables to be run on the defined interval.
func NewRunner(
	interval time.Duration,
	runnables ...Runnable,
) *Runner {
	return &Runner{
		runnables: runnables,
		ticker:    time.NewTicker(interval),
	}
}

// Runnables must implement this interface.
type Runnable interface {
	// called once at Start() time
	Setup() error
	// called on the interval defined by the
	// duration passed into NewRunner
	Run() error
}

// Call this to setup() then have
func (r *Runner) Start() error {
	err := r.setup()
	if err != nil {
		return err
	}
	err = r.run()
	if err != nil {
		return err
	}
	return nil
}

func (r *Runner) setup() error {
	for _, r := range r.runnables {
		err := r.Setup()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) run() error {
	for range r.ticker.C {
		for _, runnable := range r.runnables {
			err := runnable.Run()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Runner) Stop() {
	r.ticker.Stop()
}
