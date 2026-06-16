// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sizeprocessor // import "github.com/multiplayer-app/opentelemetry-collector-contrib/processor/sizeprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logSizeProcessor struct {
	logger *zap.Logger
}

func newLogSizeProcessor(logger *zap.Logger) *logSizeProcessor {
	return &logSizeProcessor{
		logger: logger,
	}
}

func calculateLogSize(lr plog.LogRecord) (int, error) {
	size := sizeOf(lr)
	// if err != nil {
	// 	return 0, err
	// }
	return size, nil
}

func (a *logSizeProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rs := rls.At(i)
		ilss := rs.ScopeLogs()
		// resource := rs.Resource()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			logs := ils.LogRecords()
			// library := ils.Scope()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)

				size, err := calculateLogSize(lr)
				if err != nil {
					continue
				}

				lr.Attributes().PutInt("log.size", int64(size))
			}
		}
	}

	return ld, nil
}
