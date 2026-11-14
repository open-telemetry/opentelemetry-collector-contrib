// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

func TestConcurrentTextDecoding(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Encoding = "iso-2022-jp"
	cfg.UnmarshalingSeparator = ""
	ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(t.Context())) })
	enc, err := textutils.LookupEncoding(cfg.Encoding)
	require.NoError(t, err)
	const expected = "日本語のログメッセージ"
	input, err := enc.NewEncoder().Bytes([]byte(expected))
	require.NoError(t, err)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 100 {
				logs, decodeErr := ext.(*textExtension).UnmarshalLogs(input)
				if !assert.NoError(t, decodeErr) || !assert.Equal(t, 1, logs.LogRecordCount()) {
					return
				}
				assert.Equal(t, expected, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
			}
		})
	}
	workers.Wait()
}
