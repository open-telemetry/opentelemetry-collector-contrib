// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracegen

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Config describes the test scenario.
type Config struct {
	WorkerCount      int
	NumTraces        int
	PropagateContext bool
	Rate             int64
	TotalDuration    time.Duration
	ServiceName      string

	// OTLP config
	Endpoint string
	Insecure bool
}

// Flags registers config flags.
func (c *Config) Flags(fs *flag.FlagSet) {
	fs.IntVar(&c.WorkerCount, "workers", 1, "Number of workers (goroutines) to run")
	fs.IntVar(&c.NumTraces, "traces", 1, "Number of traces to generate in each worker (ignored if duration is provided")
	fs.BoolVar(&c.PropagateContext, "marshal", false, "Whether to marshal trace context via HTTP headers")
	fs.Int64Var(&c.Rate, "rate", 0, "Approximately how many traces per second each worker should generate. Zero means no throttling.")
	fs.DurationVar(&c.TotalDuration, "duration", 0, "For how long to run the test")
	fs.StringVar(&c.ServiceName, "service", "tracegen", "Service name to use")

	// unfortunately, at this moment, the otel-go client doesn't support configuring OTLP via env vars
	fs.StringVar(&c.Endpoint, "otlp-endpoint", "localhost:4317", "Target to which the exporter is going to send spans or metrics. This MAY be configured to include a path (e.g. example.com/v1/traces)")
	fs.BoolVar(&c.Insecure, "otlp-insecure", false, "Whether to enable client transport security for the exporter's grpc or http connection")
}

// Run executes the test scenario.
func Run(c *Config, logger *zap.Logger) error {
	if c.TotalDuration > 0 {
		c.NumTraces = 0
	} else if c.NumTraces <= 0 {
		return fmt.Errorf("either `traces` or `duration` must be greater than 0")
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of traces isn't being throttled")
	} else {
		logger.Info("generation of traces is limited", zap.Float64("per-second", float64(limit)))
	}

	wg := sync.WaitGroup{}
	var running uint32 = 1
	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numTraces:        c.NumTraces,
			propagateContext: c.PropagateContext,
			limitPerSecond:   limit,
			totalDuration:    c.TotalDuration,
			running:          &running,
			wg:               &wg,
			logger:           logger.With(zap.Int("worker", i)),
		}

		go w.simulateTraces()
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		atomic.StoreUint32(&running, 0)
	}
	wg.Wait()
	return nil
}
