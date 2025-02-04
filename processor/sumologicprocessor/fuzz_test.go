// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func FuzzProcessTraces(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte, processorType uint8) {
		ju := &ptrace.JSONUnmarshaler{}
		traces, err := ju.UnmarshalTraces(data)
		if err != nil {
			return
		}
		switch int(processorType) % 4 {
		case 0:
			proc := &aggregateAttributesProcessor{}
			_ = proc.processTraces(traces)
		case 1:
			proc := &cloudNamespaceProcessor{}
			_ = proc.processTraces(traces)
		case 2:
			proc := &NestingProcessor{}
			_ = proc.processTraces(traces)
		case 3:
			proc := &translateAttributesProcessor{}
			_ = proc.processTraces(traces)
		}
	})
}

func FuzzProcessLogs(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte, processorType uint8) {
		ju := &plog.JSONUnmarshaler{}
		logs, err := ju.UnmarshalLogs(data)
		if err != nil {
			return
		}
		switch int(processorType) % 4 {
		case 0:
			proc := &aggregateAttributesProcessor{}
			_ = proc.processLogs(logs)
		case 1:
			proc := &cloudNamespaceProcessor{}
			_ = proc.processLogs(logs)
		case 2:
			proc := &NestingProcessor{}
			_ = proc.processLogs(logs)
		case 3:
			proc := &translateAttributesProcessor{}
			_ = proc.processLogs(logs)
		}
	})
}

func FuzzProcessMetrics(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte, processorType uint8) {
		ju := &pmetric.JSONUnmarshaler{}
		metrics, err := ju.UnmarshalMetrics(data)
		if err != nil {
			return
		}
		switch int(processorType) % 4 {
		case 0:
			proc := &aggregateAttributesProcessor{}
			_ = proc.processMetrics(metrics)
		case 1:
			proc := &cloudNamespaceProcessor{}
			_ = proc.processMetrics(metrics)
		case 2:
			proc := &NestingProcessor{}
			_ = proc.processMetrics(metrics)
		case 3:
			proc := &translateAttributesProcessor{}
			_ = proc.processMetrics(metrics)
		}
	})
}
