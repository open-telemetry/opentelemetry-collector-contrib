// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

func TestExternalMessageDecoder(t *testing.T) {
	var decoder encoding.MetricsMessageDecoderExtension = externalMetricsDecoder{}
	metrics, err := decoder.DecodeMetrics(t.Context(), []byte("test.metric"), encoding.WithMessageMetadata(
		client.NewMetadata(map[string][]string{"x-tenant": {"tenant-a", "tenant-b"}}),
	))
	require.NoError(t, err)

	resourceMetrics := metrics.ResourceMetrics().At(0)
	tenant, ok := resourceMetrics.Resource().Attributes().Get("tenant")
	require.True(t, ok)
	assert.Equal(t, "tenant-a,tenant-b", tenant.Str())
	assert.Equal(t, "test.metric", resourceMetrics.ScopeMetrics().At(0).Metrics().At(0).Name())
}

type externalMetricsDecoder struct {
	extension.Extension
}

func (externalMetricsDecoder) DecodeMetrics(ctx context.Context, payload []byte, opts ...encoding.MetricsDecodeOption) (pmetric.Metrics, error) {
	config, err := encoding.NewDecodeConfig(opts...)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	if err := ctx.Err(); err != nil {
		return pmetric.Metrics{}, err
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resourceMetrics.Resource().Attributes().PutStr("tenant", strings.Join(config.MessageMetadata.Get("x-tenant"), ","))
	resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(string(payload))
	return metrics, nil
}
