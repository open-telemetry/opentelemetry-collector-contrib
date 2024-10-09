// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"context"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"testing"
)

func FuzzConsumeLogs(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &plog.JSONUnmarshaler{}
		logs, err := ju.UnmarshalLogs(data)
		if err != nil {
			return
		}
		sink := new(consumertest.LogsSink)
		set := processortest.NewNopSettings()
		cfg := &Config{}
		lp, err := newProcessor(cfg, sink, set)
		if err != nil {
			t.Fatal(err)
		}
		_ = lp.ConsumeLogs(context.Background(), logs)
	})
}
