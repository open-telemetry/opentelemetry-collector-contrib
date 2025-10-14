// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regex

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func BenchmarkProcessBatch(b *testing.B) {
	b.Run("SeverityMapping", func(b *testing.B) {
		config := NewConfig()
		config.OnError = helper.SendOnError
		config.Regex = `(?P<remote_host>[^\s]+) - (?P<remote_user>[^\s]+) \[(?P<timestamp>[^\]]+)\] "(?P<http_method>[A-Z]+) (?P<path>[^\s]+) [^"]+" (?P<http_status>\d+) (?P<bytes_sent>[^\s]+)`
		config.TimeParser = &helper.TimeParser{
			ParseFrom:  func() *entry.Field { f := entry.NewAttributeField("timestamp"); return &f }(),
			Layout:     "%d/%b/%Y:%H:%M:%S %z",
			LayoutType: helper.StrptimeKey,
		}
		config.SeverityConfig = &helper.SeverityConfig{
			ParseFrom: func() *entry.Field { f := entry.NewAttributeField("http_status"); return &f }(),
			Mapping: map[string]any{
				"critical": "5xx",
				"error":    "4xx",
				"info":     "3xx",
				"debug":    "2xx",
			},
		}

		op, err := config.Build(componenttest.NewNopTelemetrySettings())
		require.NoError(b, err)

		entries := make([]*entry.Entry, 1000000)
		for i := range 1000 {
			entries[i] = entry.New()
			entries[i].Body = fmt.Sprintf("10.33.121.119 - - [11/Aug/2020:00:00:00 -0400] \"GET /index.html HTTP/1.1\" 404 %d\n", i%1000)
		}

		for b.Loop() {
			require.NoError(b, op.ProcessBatch(b.Context(), entries))
		}
	})
}
