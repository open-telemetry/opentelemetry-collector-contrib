// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/traces"

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

type worker struct {
	running          *atomic.Bool          // pointer to shared flag that indicates it's time to stop the test
	numTraces        int                   // how many traces the worker has to generate (only when duration==0)
	numChildSpans    int                   // how many child spans the worker has to generate per trace
	propagateContext bool                  // whether the worker needs to propagate the trace context via HTTP headers
	statusCode       codes.Code            // the status code set for the child and parent spans
	totalDuration    types.DurationWithInf // how long to run the test for (overrides `numTraces`)
	limitPerSecond   rate.Limit            // how many spans per second to generate
	wg               *sync.WaitGroup       // notify when done
	loadSize         int                   // desired minimum size in MB of string data for each generated trace
	spanDuration     time.Duration         // duration of generated spans
	numSpanLinks     int                   // number of span links to generate per span
	logger           *zap.Logger
	allowFailures    bool                // whether to continue on export failures
	spanContexts     []trace.SpanContext // collection of span contexts for linking
	spanContextsMu   sync.RWMutex        // mutex for spanContexts slice
}

const (
	fakeIP                string = "1.2.3.4"
	maxSpanContextsBuffer int    = 1000 // Maximum number of span contexts to keep for linking
)

// addSpanContext safely adds a span context to the worker's collection
// Maintains a circular buffer to prevent unbounded memory growth
func (w *worker) addSpanContext(spanCtx trace.SpanContext) {
	w.spanContextsMu.Lock()
	defer w.spanContextsMu.Unlock()

	w.spanContexts = append(w.spanContexts, spanCtx)

	// Keep only the most recent span contexts to prevent memory growth
	if len(w.spanContexts) > maxSpanContextsBuffer {
		w.spanContexts = w.spanContexts[1 : maxSpanContextsBuffer+1]
	}
}

// generateSpanLinks creates span links to random existing span contexts
func (w *worker) generateSpanLinks() []trace.Link {
	if w.numSpanLinks <= 0 {
		return nil
	}

	w.spanContextsMu.RLock()
	defer w.spanContextsMu.RUnlock()

	availableContexts := len(w.spanContexts)
	if availableContexts == 0 {
		return nil
	}

	links := make([]trace.Link, 0, w.numSpanLinks)

	// Generate links to random existing span contexts
	for i := 0; i < w.numSpanLinks; i++ {
		randomIndex := rand.IntN(availableContexts)
		spanCtx := w.spanContexts[randomIndex]

		links = append(links, trace.Link{
			SpanContext: spanCtx,
			Attributes: []attribute.KeyValue{
				attribute.String("link.type", "random"),
				attribute.Int("link.index", i),
			},
		})
	}

	return links
}

func (w *worker) simulateTraces(telemetryAttributes []attribute.KeyValue) {
	tracer := otel.Tracer("telemetrygen")
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int

	for w.running.Load() {
		spanStart := time.Now()
		spanEnd := spanStart.Add(w.spanDuration)

		if err := limiter.Wait(context.Background()); err != nil {
			w.logger.Fatal("limiter waited failed, retry", zap.Error(err))
		}

		// Generate span links for the parent span
		parentLinks := w.generateSpanLinks()

		ctx, sp := tracer.Start(context.Background(), "lets-go", trace.WithAttributes(
			semconv.NetworkPeerAddress(fakeIP),
			semconv.PeerService("telemetrygen-server"),
		),
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithTimestamp(spanStart),
			trace.WithLinks(parentLinks...),
		)
		sp.SetAttributes(telemetryAttributes...)
		for j := 0; j < w.loadSize; j++ {
			sp.SetAttributes(common.CreateLoadAttribute(fmt.Sprintf("load-%v", j), 1))
		}

		// Store the parent span context for potential future linking
		w.addSpanContext(sp.SpanContext())

		childCtx := ctx
		if w.propagateContext {
			header := propagation.HeaderCarrier{}
			// simulates going remote
			otel.GetTextMapPropagator().Inject(childCtx, header)

			// simulates getting a request from a client
			childCtx = otel.GetTextMapPropagator().Extract(childCtx, header)
		}
		var endTimestamp trace.SpanEventOption

		for j := 0; j < w.numChildSpans; j++ {
			if err := limiter.Wait(context.Background()); err != nil {
				w.logger.Fatal("limiter waited failed, retry", zap.Error(err))
			}

			// Generate span links for child spans
			childLinks := w.generateSpanLinks()

			_, child := tracer.Start(childCtx, "okey-dokey-"+strconv.Itoa(j), trace.WithAttributes(
				semconv.NetworkPeerAddress(fakeIP),
				semconv.PeerService("telemetrygen-client"),
			),
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithTimestamp(spanStart),
				trace.WithLinks(childLinks...),
			)
			child.SetAttributes(telemetryAttributes...)

			// Store the child span context for potential future linking
			w.addSpanContext(child.SpanContext())

			endTimestamp = trace.WithTimestamp(spanEnd)
			child.SetStatus(w.statusCode, "")
			child.End(endTimestamp)

			// Reset the start and end for next span
			spanStart = spanEnd
			spanEnd = spanStart.Add(w.spanDuration)
		}
		sp.SetStatus(w.statusCode, "")
		sp.End(endTimestamp)

		i++
		if w.numTraces != 0 {
			if i >= w.numTraces {
				break
			}
		}
	}
	w.logger.Info("traces generated", zap.Int("traces", i))
	w.wg.Done()
}
