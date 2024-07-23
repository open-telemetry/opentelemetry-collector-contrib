// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampk8sobjectsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sobjectsprocessor"

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type opsrampK8sObjectsProcessor struct {
	logger *zap.Logger
}

// newOpsrampK8sObjectsProcessor returns a processor
func newOpsrampK8sObjectsProcessor(logger *zap.Logger) *opsrampK8sObjectsProcessor {
	return &opsrampK8sObjectsProcessor{
		logger: logger,
	}
}

func (a *opsrampK8sObjectsProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	out := make(map[string]interface{})
	addedUids := make(map[string]interface{})
	outld := plog.NewLogs()
	isWatchLog := true

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rs := rls.At(i)
		ilss := rs.ScopeLogs()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			logs := ils.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)

				var watchevent struct {
					Object struct {
						Metadata struct {
							Uid string `json:"uid"`
						} `json:"metadata"`
					} `json:"object"`
					Type string `json:"type"`
				}

				err := json.Unmarshal([]byte(lr.Body().AsString()), &watchevent)
				if err != nil {
					a.logger.Error("Error unmarshalling json", zap.Error(err))
					continue
					// Throw exception and not continue. may be assert.
				}

				uid := watchevent.Object.Metadata.Uid
				if uid == "" {
					isWatchLog = false
					break
				}

				eventType := watchevent.Type

				if eventType == "ADDED" {
					addedUids[uid] = nil
				}

				/*
					if eventType == "MODIFIED" {
						if _, exists := addedUids[uid]; exists {
							watchevent.Type = "ADDED"
						}
					}
				*/

				if eventType == "DELETED" {
					if _, exists := addedUids[uid]; exists {
						delete(out, uid)
						continue
					}
				}

				out[watchevent.Object.Metadata.Uid] = lr
			}

			if !isWatchLog {
				break
			}
		}

		if !isWatchLog {
			isWatchLog = true
			resourceLogs := outld.ResourceLogs()
			dst := resourceLogs.AppendEmpty()
			rs.CopyTo(dst)
		}
	}

	/*
		fmt.Println("Distinct event size\n", len(out))
		for k, v := range out {
			fmt.Println("Hi")
			lr := v.(plog.LogRecord)
			fmt.Printf("UID : %s, Data : %v\n", k, lr.Body().AsString())
		}
		// Sleep required because some times the print logs are not seen. Mosly processor is died before it prints/flushes.
		time.Sleep(1 * time.Second)
	*/

	resourceLogs := outld.ResourceLogs()

	for _, v := range out {
		rl := resourceLogs.AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		logSlice := sl.LogRecords()
		dstLogRecord := logSlice.AppendEmpty()
		srcLogRecord := v.(plog.LogRecord)
		srcLogRecord.CopyTo(dstLogRecord)
	}

	return outld, nil
}
