// Copyright 2022 The OpenTelemetry Authors
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

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"go.uber.org/zap"
	zapObserver "go.uber.org/zap/zaptest/observer"
)

func TestStart(t *testing.T) {
	// prepare
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	sink := &consumertest.MetricsSink{}
	recv := newReceiver(cfg, receivertest.NewNopCreateSettings(), sink)

	// test
	err := recv.Start(context.Background(), componenttest.NewNopHost())

	// verify
	assert.NoError(t, err)
}

func TestShutdown(t *testing.T) {
	// prepare
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	sink := &consumertest.MetricsSink{}
	recv := newReceiver(cfg, receivertest.NewNopCreateSettings(), sink)

	err := recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	err = recv.Shutdown(context.Background())

	// verify
	assert.NoError(t, err)
}

func TestLoggingHost(t *testing.T) {
	core, obs := zapObserver.New(zap.ErrorLevel)
	host := &loggingHost{
		Host:   componenttest.NewNopHost(),
		logger: zap.New(core),
	}
	host.ReportFatalError(errors.New("runtime error"))
	require.Equal(t, 1, obs.Len())
	log := obs.All()[0]
	assert.Equal(t, "receiver reported a fatal error", log.Message)
	assert.Equal(t, "runtime error", log.ContextMap()["error"])
}
