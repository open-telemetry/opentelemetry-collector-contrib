// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package interval is a work in progress. Later versions may support parallel
// execution.
package interval

import "time"

// Runner takes a list of `Runnable`s, calls Setup() on each of them and then
// calls Run() on each of them, using a Ticker, sequentially and within the same
// goroutine. Call Stop() to turn off the Ticker.
type Runner struct {
	runnables []Runnable
	ticker    *time.Ticker
}

// NewRunner creates a new interval runner. Pass in a duration (time between
// calls) and one or more Runnables to be run on the defined interval.
func NewRunner(interval time.Duration, runnables ...Runnable) *Runner {
	return &Runner{
		runnables: runnables,
		ticker:    time.NewTicker(interval),
	}
}

// Runnable must be implemented by types passed into the Runner constructor.
type Runnable interface {
	// called once at Start() time
	Setup() error
	// called on the interval defined by the
	// duration passed into NewRunner
	Run() error
}

// Start kicks off this Runner. Calls Setup() and Run() on the passed-in
// Runnables.
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
	for _, runnable := range r.runnables {
		err := runnable.Setup()
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

// Stop turns off this Runner's ticker.
func (r *Runner) Stop() {
	r.ticker.Stop()
}
