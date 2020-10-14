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
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/propagation"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/semconv"
	"golang.org/x/time/rate"
)

type worker struct {
	running          *uint32         // pointer to shared flag that indicates it's time to stop the test
	id               int             // worker id
	numTraces        int             // how many traces the worker has to generate (only when duration==0)
	propagateContext bool            // whether the worker needs to propagate the trace context via HTTP headers
	totalDuration    time.Duration   // how long to run the test for (overrides `numTraces`)
	limitPerSecond   rate.Limit      // how many spans per second to generate
	wg               *sync.WaitGroup // notify when done
	logger           logr.Logger
}

const (
	fakeIP string = "1.2.3.4"

	fakeSpanDuration = 123 * time.Microsecond
)

func (w worker) simulateTraces() {
	tracer := global.Tracer("tracegen")
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int
	for atomic.LoadUint32(w.running) == 1 {
		ctx, sp := tracer.Start(context.Background(), "lets-go", trace.WithAttributes(
			label.String("span.kind", "client"), // is there a semantic convention for this?
			semconv.NetPeerIPKey.String(fakeIP),
			semconv.PeerServiceKey.String("tracegen-server"),
		))

		childCtx := ctx
		if w.propagateContext {
			header := http.Header{}
			// simulates going remote
			propagation.InjectHTTP(childCtx, global.Propagators(), header)

			// simulates getting a request from a client
			childCtx = propagation.ExtractHTTP(childCtx, global.Propagators(), header)
		}

		_, child := tracer.Start(childCtx, "okey-dokey", trace.WithAttributes(
			label.String("span.kind", "server"),
			semconv.NetPeerIPKey.String(fakeIP),
			semconv.PeerServiceKey.String("tracegen-client"),
		))

		limiter.Wait(context.Background())

		opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
		child.End(opt)
		sp.End(opt)

		i++
		if w.numTraces != 0 {
			if i >= w.numTraces {
				break
			}
		}
	}
	w.logger.Info("traces generated", "worker", w.id, "traces", i)
	w.wg.Done()
}
