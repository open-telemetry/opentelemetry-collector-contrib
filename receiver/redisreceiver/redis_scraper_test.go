// Copyright 2020, OpenTelemetry Authors
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

package redisreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

func TestRedisRunnable(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	settings := componenttest.NewNopReceiverCreateSettings()
	settings.Logger = logger
	runner, err := newRedisScraperWithClient(newFakeClient(), settings, "TestEndpoint")
	require.NoError(t, err)
	md, err := runner.Scrape(context.Background())
	require.NoError(t, err)
	// + 6 because there are two keyspace entries each of which has three metrics
	assert.Equal(t, len(getDefaultRedisMetrics())+6, md.DataPointCount())
	rm := md.ResourceMetrics().At(0)
	value, _ := rm.Resource().Attributes().Get("redis.instance")
	assert.Equal(t, "TestEndpoint", value.AsString())
	ilm := rm.InstrumentationLibraryMetrics().At(0)
	il := ilm.InstrumentationLibrary()
	assert.Equal(t, "otelcol/redis", il.Name())
}

func TestNewReceiver_invalid_auth_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.TLS = configtls.TLSClientSetting{
		TLSSetting: configtls.TLSSetting{
			CAFile: "/invalid",
		},
	}
	r, err := createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), c, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
	assert.Nil(t, r)
}
