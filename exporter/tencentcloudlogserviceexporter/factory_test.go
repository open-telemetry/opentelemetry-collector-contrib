// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/internal/metadata"
)

func TestFactoryTypeAlias(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, component.MustNewType("tencent_cloud_log_service"), factory.Type())

	for _, typ := range []component.Type{metadata.Type, component.MustNewType("tencentcloud_logservice")} {
		t.Run(typ.String(), func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			comp, err := factory.CreateLogs(t.Context(), exportertest.NewNopSettings(typ), cfg)
			require.NoError(t, err)
			require.NotNil(t, comp)
			require.NoError(t, comp.Shutdown(t.Context()))
		})
	}
}
