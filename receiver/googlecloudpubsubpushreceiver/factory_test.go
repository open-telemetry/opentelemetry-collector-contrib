// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver/internal/metadata"
)

func TestFactoryTypeAlias(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, component.MustNewType("google_cloud_pub_sub_push"), factory.Type())

	for _, typ := range []component.Type{metadata.Type, component.MustNewType("googlecloudpubsubpush")} {
		t.Run(typ.String(), func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			comp, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(typ), cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NotNil(t, comp)
			require.NoError(t, comp.Shutdown(t.Context()))
		})
	}
}
