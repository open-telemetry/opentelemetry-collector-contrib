// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

type progressReporter struct {
	name       string
	totalBytes int64
	totalRows  int64
	interval   time.Duration
	logger     *zap.Logger
}

func newProgressReporter(name string, interval int, logger *zap.Logger) *progressReporter {
	return &progressReporter{
		name:       name,
		totalBytes: 0,
		totalRows:  0,
		interval:   time.Duration(interval) * time.Second,
		logger:     logger,
	}
}

func (reporter *progressReporter) incrTotalBytes(bytes int64) {
	atomic.AddInt64(&reporter.totalBytes, bytes)
}

func (reporter *progressReporter) incrTotalRows(rows int64) {
	atomic.AddInt64(&reporter.totalRows, rows)
}

func (reporter *progressReporter) report() {
	initTime := time.Now().Unix()
	lastTime := initTime
	lastBytes := atomic.LoadInt64(&reporter.totalBytes)
	lastRows := atomic.LoadInt64(&reporter.totalRows)

	reporter.logger.Info(fmt.Sprintf("start [%v] progress reporter with interval %v", reporter.name, reporter.interval))
	for reporter.interval > 0 {
		time.Sleep(reporter.interval)

		curTime := time.Now().Unix()
		curBytes := atomic.LoadInt64(&reporter.totalBytes)
		curRows := atomic.LoadInt64(&reporter.totalRows)
		totalTime := curTime - initTime
		totalSpeedMbps := curBytes / 1024 / 1024 / totalTime
		totalSpeedRps := curRows / totalTime

		incBytes := curBytes - lastBytes
		incRows := curRows - lastRows
		incTime := curTime - lastTime
		incSpeedMbps := incBytes / 1024 / 1024 / incTime
		incSpeedRps := incRows / incTime

		reporter.logger.Info(fmt.Sprintf("[%v] total %v MB %v ROWS, total speed %v MB/s %v R/s, last %v seconds speed %v MB/s %v R/s",
			reporter.name,
			curBytes/1024/1024, curRows, totalSpeedMbps, totalSpeedRps,
			incTime, incSpeedMbps, incSpeedRps))

		lastTime = curTime
		lastBytes = curBytes
		lastRows = curRows
	}
}
