// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor/internal/metadata"
)

func FuzzConsumeLogs(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &plog.JSONUnmarshaler{}
		logs, err := ju.UnmarshalLogs(data)
		if err != nil {
			return
		}
		sink := new(consumertest.LogsSink)
		set := processortest.NewNopSettings(metadata.Type)
		cfg := &Config{}
		lp, err := newProcessor(cfg, sink, set)
		if err != nil {
			t.Fatal(err)
		}
		_ = lp.ConsumeLogs(context.Background(), logs)
	})
}
