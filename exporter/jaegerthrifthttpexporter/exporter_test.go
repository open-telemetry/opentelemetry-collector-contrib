// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerthrifthttpexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const testHTTPAddress = "http://a.example.com:123/at/some/path"

func TestNew(t *testing.T) {
	config := Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: testHTTPAddress,
			Headers:  map[string]configopaque.String{"test": "test"},
			Timeout:  10 * time.Nanosecond,
		},
	}

	got, err := newTracesExporter(&config, exportertest.NewNopCreateSettings())
	assert.NoError(t, err)
	require.NotNil(t, got)

	err = got.ConsumeTraces(context.Background(), ptrace.NewTraces())
	assert.NoError(t, err)
}
