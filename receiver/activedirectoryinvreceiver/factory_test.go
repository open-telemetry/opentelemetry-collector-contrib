// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectoryinvreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.EqualValues(t, metadata.Type, ft)
}

func TestCreateLogsReceiver(t *testing.T) {
	f := NewFactory()
	cfg := createDefaultConfig().(*ADConfig)
	_, err := NewFactory().CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(f.Type()),
		cfg,
		nil,
	)
	require.NoError(t, err)
}
