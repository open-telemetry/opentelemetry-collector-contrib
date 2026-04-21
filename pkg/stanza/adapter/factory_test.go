// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/jsonparser"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
)

func TestCreateReceiver(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)
		cfg := factory.CreateDefaultConfig().(*TestConfig)
		cfg.Operators = []operator.Config{
			{
				Builder: jsonparser.NewConfig(),
			},
		}
		receiver, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(factory.Type()), cfg, consumertest.NewNop())
		require.NoError(t, err, "receiver creation failed")
		require.NotNil(t, receiver, "receiver creation failed")
	})

	t.Run("DecodeOperatorConfigsFailureMissingFields", func(t *testing.T) {
		factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)
		badCfg := factory.CreateDefaultConfig().(*TestConfig)
		badCfg.Operators = []operator.Config{
			{
				Builder: regex.NewConfig(),
			},
		}
		receiver, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(factory.Type()), badCfg, consumertest.NewNop())
		require.Error(t, err, "receiver creation should fail if parser configs aren't valid")
		require.Nil(t, receiver, "receiver creation should fail if parser configs aren't valid")
	})
}
