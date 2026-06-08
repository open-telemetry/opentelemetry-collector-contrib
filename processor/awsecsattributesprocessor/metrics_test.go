// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConsumeMetrics(t *testing.T) {
	srv := newMetadataServer(t)
	cfg := defaultTestConfig()
	require.NoError(t, cfg.init())
	p := newMetricsProcessor(zaptestLogger(t), cfg, consumertest.NewNop(), staticEndpoints(srv.URL))

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("container.id", testContainerID)

	require.NoError(t, p.ConsumeMetrics(t.Context(), md))

	attrs := md.ResourceMetrics().At(0).Resource().Attributes()
	require.Equal(t, len(expectedFlattenedMetadata)+1, attrs.Len())
	v, ok := attrs.Get("aws.ecs.cluster")
	require.True(t, ok)
	require.Equal(t, "cds-305", v.AsString())
}
