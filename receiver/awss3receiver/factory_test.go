// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "aws_s3", factory.Type().String())
}

// TestCreateReceiverWithDeprecatedTypeAlias verifies that the receiver can still be
// configured using the deprecated `awss3` type in addition to the current `aws_s3` type.
func TestCreateReceiverWithDeprecatedTypeAlias(t *testing.T) {
	for _, typ := range []component.Type{metadata.Type, metadata.DeprecatedType} {
		t.Run(typ.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.S3Downloader.S3Bucket = "abucket"
			cfg.StartTime = "2024-01-01"
			cfg.EndTime = "2024-01-02"

			set := receivertest.NewNopSettings(factory.Type())
			set.ID = component.NewID(typ)

			traces, err := factory.CreateTraces(t.Context(), set, cfg, consumertest.NewNop())
			require.NoError(t, err)
			assert.NotNil(t, traces)

			metrics, err := factory.CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
			require.NoError(t, err)
			assert.NotNil(t, metrics)

			logs, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
			require.NoError(t, err)
			assert.NotNil(t, logs)
		})
	}
}
