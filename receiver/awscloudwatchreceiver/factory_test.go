// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.Equal(t, metadata.Type, ft)
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-west-2"
	_, err := NewFactory().CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		nil,
	)
	require.NoError(t, err)
}
