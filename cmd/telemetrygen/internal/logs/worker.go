// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type worker struct {
	running        *atomic.Bool    // pointer to shared flag that indicates it's time to stop the test
	numLogs        int             // how many logs the worker has to generate (only when duration==0)
	totalDuration  time.Duration   // how long to run the test for (overrides `numLogs`)
	limitPerSecond rate.Limit      // how many logs per second to generate
	wg             *sync.WaitGroup // notify when done
	logger         *zap.Logger     // logger
	index          int             // worker index
}

var reRandom = regexp.MustCompile(`\${random:(\d+)-(\d+)}`)

func replaceRandoms(inp string) string {
	indices := reRandom.FindStringSubmatch(inp)
	lower, err := strconv.Atoi(indices[1])
	if err != nil {
		panic(err)
	}
	upper, err := strconv.Atoi(indices[2])
	if err != nil {
		panic(err)
	}
	num := lower + rand.Intn(upper-lower)

	return fmt.Sprintf("%d", num)
}

func (w worker) simulateLogs(res *resource.Resource, exporter exporter) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int64

	for w.running.Load() {
		logs := plog.NewLogs()
		nRes := logs.ResourceLogs().AppendEmpty().Resource()
		attrs := res.Attributes()
		for _, attr := range attrs {
			val := attr.Value.AsString()
			val = reRandom.ReplaceAllStringFunc(val, replaceRandoms)
			nRes.Attributes().PutStr(string(attr.Key), val)
		}
		log := logs.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		log.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		log.SetDroppedAttributesCount(1)
		log.SetSeverityNumber(plog.SeverityNumberInfo)
		log.SetSeverityText("Info")
		lattrs := log.Attributes()
		lattrs.PutStr("app", "server")

		if err := exporter.export(logs); err != nil {
			w.logger.Fatal("exporter failed", zap.Error(err))
		}
		if err := limiter.Wait(context.Background()); err != nil {
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		i++
		if w.numLogs != 0 && i >= int64(w.numLogs) {
			break
		}
	}

	w.logger.Info("logs generated", zap.Int64("logs", i))
	w.wg.Done()
}
