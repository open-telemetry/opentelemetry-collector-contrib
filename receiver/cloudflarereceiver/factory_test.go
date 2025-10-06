// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.Equal(t, metadata.Type, ft)
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	_, err := NewFactory().CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		nil,
	)
	require.NoError(t, err)
}
