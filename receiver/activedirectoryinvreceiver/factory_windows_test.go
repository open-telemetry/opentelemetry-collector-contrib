// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package activedirectoryinvreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateLogsReceiver(t *testing.T) {
	f := NewFactory()
	cfg := createDefaultConfig().(*ADConfig)
	_, err := f.CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(f.Type()),
		cfg,
		nil,
	)
	require.NoError(t, err)
}
