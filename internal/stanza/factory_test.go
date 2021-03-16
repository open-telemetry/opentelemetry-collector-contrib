// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stanza

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type factoryTestCase struct {
	name      string
	cfgMod    cfgModFunc
	expectErr bool
}

type cfgModFunc func(*TestConfig) *TestConfig

type validationFunc func()

func TestCreateReceiver(t *testing.T) {

	testCases := []factoryTestCase{
		{
			name: "Simple",
			cfgMod: func(cfg *TestConfig) *TestConfig {
				cfg.Operators = []map[string]interface{}{
					{
						"type": "json_parser",
					},
				}
				return cfg
			},
		},
		{
			name: "OffsetsFileDisabledDefault",
			cfgMod: func(cfg *TestConfig) *TestConfig {
				cfg.Operators = []map[string]interface{}{
					{
						"type": "json_parser",
					},
				}
				cfg.OffsetsFile = OffsetsConfig{
					Enabled: false,
				}
				return cfg
			},
		},
		{
			name: "OffsetsFileDisabledCustom",
			cfgMod: func(cfg *TestConfig) *TestConfig {
				cfg.Operators = []map[string]interface{}{
					{
						"type": "json_parser",
					},
				}
				cfg.OffsetsFile = OffsetsConfig{
					Enabled: false,
					Path:    filepath.Join(newTempDir(t), "offsets.db"),
				}
				return cfg
			},
		},
		{
			name: "OffsetsFileEnabledDefault",
			cfgMod: func(cfg *TestConfig) *TestConfig {
				cfg.Operators = []map[string]interface{}{
					{
						"type": "json_parser",
					},
				}
				cfg.OffsetsFile = OffsetsConfig{
					Enabled: true,
				}
				return cfg
			},
		},
		{
			name: "OffsetsFileEnabledCustom",
			cfgMod: func(cfg *TestConfig) *TestConfig {
				cfg.Operators = []map[string]interface{}{
					{
						"type": "json_parser",
					},
				}
				cfg.OffsetsFile = OffsetsConfig{
					Enabled: true,
					Path:    filepath.Join(newTempDir(t), "offsets.db"),
				}
				return cfg
			},
		},
		{
			name: "DecodeInputConfigFailure",
			cfgMod: func(cfg *TestConfig) *TestConfig {
				cfg.Input = map[string]interface{}{
					"type": "unknown",
				}
				return cfg
			},
			expectErr: true,
		},
		{
			name: "DecodeOperatorConfigsFailureMissingFields",
			cfgMod: func(cfg *TestConfig) *TestConfig {
				cfg.Operators = []map[string]interface{}{
					{
						"badparam": "badvalue",
					},
				}
				return cfg
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		runCreateTest(t, tc)
	}
}

func runCreateTest(t *testing.T, tc factoryTestCase) {
	factory := NewFactory(TestReceiverType{})
	cfg := factory.CreateDefaultConfig().(*TestConfig)
	cfg = tc.cfgMod(cfg)
	receiver, err := factory.CreateLogsReceiver(context.Background(), params(), cfg, &mockLogsConsumer{})

	if tc.expectErr {
		require.Error(t, err)
		require.Nil(t, receiver)
	} else {
		require.NoError(t, err)
		require.NotNil(t, receiver)
	}
}

func TestSuccessWithDifferentOffsetsFiles(t *testing.T) {
	tempDir := newTempDir(t)
	factory := NewFactory(TestReceiverType{})
	cfg := factory.CreateDefaultConfig().(*TestConfig)
	cfg.Operators = []map[string]interface{}{
		{
			"type": "json_parser",
		},
	}
	cfg.OffsetsFile = OffsetsConfig{true, filepath.Join(tempDir, "one.db")}
	receiver, err := factory.CreateLogsReceiver(context.Background(), params(), cfg, &mockLogsConsumer{})
	require.NoError(t, err, "receiver creation failed")
	require.NotNil(t, receiver, "receiver creation failed")

	cfg.OffsetsFile = OffsetsConfig{true, filepath.Join(tempDir, "two.db")}
	receiver, err = factory.CreateLogsReceiver(context.Background(), params(), cfg, &mockLogsConsumer{})
	require.NoError(t, err, "receiver creation failed")
	require.NotNil(t, receiver, "receiver creation failed")
}

// This behavior is not ideal, but it is currently expected
func TestFailureWithReusedOffsetsFile(t *testing.T) {
	tempDir := newTempDir(t)
	factory := NewFactory(TestReceiverType{})
	cfg := factory.CreateDefaultConfig().(*TestConfig)
	cfg.Operators = []map[string]interface{}{
		{
			"type": "json_parser",
		},
	}
	cfg.OffsetsFile = OffsetsConfig{true, filepath.Join(tempDir, "offsets.db")}
	receiver, err := factory.CreateLogsReceiver(context.Background(), params(), cfg, &mockLogsConsumer{})
	require.NoError(t, err, "receiver creation failed")
	require.NotNil(t, receiver, "receiver creation failed")

	receiver, err = factory.CreateLogsReceiver(context.Background(), params(), cfg, &mockLogsConsumer{})
	require.Error(t, err, "receiver creation fails due to open offsets file")
}

func params() component.ReceiverCreateParams {
	return component.ReceiverCreateParams{
		Logger: zap.NewNop(),
	}
}
