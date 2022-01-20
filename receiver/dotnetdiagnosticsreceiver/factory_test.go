// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnetdiagnosticsreceiver

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, "dotnet_diagnostics", string(f.Type()))
	cfg := f.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
	assert.Equal(
		t,
		[]string{"System.Runtime", "Microsoft.AspNetCore.Hosting"},
		cfg.(*Config).Counters,
	)
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	params := componenttest.NewNopReceiverCreateSettings()
	r, err := f.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestMkConnectionSupplier(t *testing.T) {
	connect := mkConnectionSupplier(0, func(network, address string) (net.Conn, error) {
		return nil, nil
	}, func(pattern string) (matches []string, err error) {
		return []string{""}, nil
	})
	readWriter, err := connect()
	require.NoError(t, err)
	require.Nil(t, readWriter)
}
