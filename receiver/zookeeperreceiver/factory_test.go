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

package zookeeperreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, component.Type("zookeeper"), f.Type())

	cfg := f.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	// Assert defaults.
	assert.Equal(t, 10*time.Second, rCfg.CollectionInterval)
	assert.Equal(t, 10*time.Second, rCfg.Timeout)
	assert.Equal(t, ":2181", rCfg.Endpoint)

	tests := []struct {
		name    string
		config  component.Config
		wantErr bool
	}{
		{
			name:   "Happy path",
			config: createDefaultConfig(),
		},
		{
			name:    "Invalid endpoint",
			config:  &Config{},
			wantErr: true,
		},
		{
			name: "Invalid timeout",
			config: &Config{
				TCPAddr: confignet.TCPAddr{
					Endpoint: ":2181",
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := f.CreateMetricsReceiver(
				context.Background(),
				receivertest.NewNopCreateSettings(),
				test.config,
				consumertest.NewNop(),
			)

			if test.wantErr {
				require.Error(t, err)
				require.Nil(t, r)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, r)
		})
	}
}
