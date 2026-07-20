// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateLogsReceiver(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	cfg := f.CreateDefaultConfig().(*Config)

	settings := receivertest.NewNopSettings(receivertest.NopType)
	settings.ID = component.MustNewID("azure_functions")
	ext, err := f.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, ext)
}
