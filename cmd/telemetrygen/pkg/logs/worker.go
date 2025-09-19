// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/log/logtest"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

type worker struct {
	running        *atomic.Bool          // pointer to shared flag that indicates it's time to stop the test
	numLogs        int                   // how many logs the worker has to generate (only when duration==0)
	body           string                // the body of the log
	severityNumber log.Severity          // the severityNumber of the log
	severityText   string                // the severityText of the log
	totalDuration  types.DurationWithInf // how long to run the test for (overrides `numLogs`)
	limitPerSecond rate.Limit            // how many logs per second to generate
	wg             *sync.WaitGroup       // notify when done
	logger         *zap.Logger           // logger
	index          int                   // worker index
	traceID        string                // traceID string
	spanID         string                // spanID string
	loadSize       int                   // desired minimum size in MB of string data for each generated log
	allowFailures  bool                  // whether to continue on export failures
}

func (w worker) simulateLogs(res *resource.Resource, exporter sdklog.Exporter, telemetryAttributes []attribute.KeyValue) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int64

	for w.running.Load() {
		var tid trace.TraceID
		var sid trace.SpanID

		if w.spanID != "" {
			// we checked this for errors in the Validate function
			b, _ := hex.DecodeString(w.spanID)
			sid = trace.SpanID(b)
		}
		if w.traceID != "" {
			// we checked this for errors in the Validate function
			b, _ := hex.DecodeString(w.traceID)
			tid = trace.TraceID(b)
		}

		attrs := []log.KeyValue{log.String("app", "server")}
		for i, attr := range telemetryAttributes {
			attrs = append(attrs, log.String(string(attr.Key), telemetryAttributes[i].Value.AsString()))
		}

		// Add load size attributes if specified
		if w.loadSize > 0 {
			for j := 0; j < w.loadSize; j++ {
				attrs = append(attrs, log.String(fmt.Sprintf("load-%v", j), string(make([]byte, common.CharactersPerMB))))
			}
		}

		rf := logtest.RecordFactory{
			Timestamp:         time.Now(),
			Severity:          w.severityNumber,
			SeverityText:      w.severityText,
			Body:              log.StringValue(w.body),
			Attributes:        attrs,
			TraceID:           tid,
			SpanID:            sid,
			Resource:          res,
			DroppedAttributes: 1,
		}

		logs := []sdklog.Record{rf.NewRecord()}

		if err := limiter.Wait(context.Background()); err != nil {
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		if err := exporter.Export(context.Background(), logs); err != nil {
			if w.allowFailures {
				w.logger.Error("exporter failed, continuing due to --allow-export-failures", zap.Error(err))
			} else {
				w.logger.Fatal("exporter failed", zap.Error(err))
			}
		}

		i++
		if w.numLogs != 0 && i >= int64(w.numLogs) {
			break
		}
	}

	w.logger.Info("logs generated", zap.Int64("logs", i))
	w.wg.Done()
}
