// Copyright 2019, OpenTelemetry Authors
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

package cloudfoundryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

// Ensure stream create works as expected
func TestValidStream(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	uaa, err := newUAATokenProvider(
		zap.NewNop(),
		cfg.UAA.LimitedHTTPClientSettings,
		cfg.UAA.Username,
		cfg.UAA.Password)

	require.NoError(t, err)
	require.NotNil(t, uaa)

	streamFactory, streamErr := newEnvelopeStreamFactory(
		zap.NewNop(),
		uaa,
		cfg.RLPGateway.HTTPClientSettings,
		componenttest.NewNopHost())

	require.NoError(t, streamErr)
	require.NotNil(t, streamFactory)

	innerCtx, cancel := context.WithCancel(context.Background())

	envelopeStream, createErr := streamFactory.CreateStream(
		innerCtx,
		cfg.RLPGateway.ShardID)

	require.NoError(t, createErr)
	require.NotNil(t, envelopeStream)

	cancel()
}

// Ensure stream create fails when it should
func TestInvalidStream(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	uaa, err := newUAATokenProvider(
		zap.NewNop(),
		cfg.UAA.LimitedHTTPClientSettings,
		cfg.UAA.Username,
		cfg.UAA.Password)

	require.NoError(t, err)
	require.NotNil(t, uaa)

	// Stream create should fail if given invalid endpoint
	cfg.RLPGateway.HTTPClientSettings.Endpoint = ""
	streamFactory, streamErr := newEnvelopeStreamFactory(
		zap.NewNop(),
		uaa,
		cfg.RLPGateway.HTTPClientSettings,
		componenttest.NewNopHost())

	require.NoError(t, streamErr)
	require.NotNil(t, streamFactory)

	innerCtx, _ := context.WithCancel(context.Background())

	envelopeStream, createErr := streamFactory.CreateStream(
		innerCtx,
		"")

	require.Error(t, createErr)
	require.Nil(t, envelopeStream)

	// Stream create should fail with invalid shardID
	INVALID_SHARD_ID := ""
	cfg.RLPGateway.HTTPClientSettings.Endpoint = defaultURL
	streamFactory, streamErr = newEnvelopeStreamFactory(
		zap.NewNop(),
		uaa,
		cfg.RLPGateway.HTTPClientSettings,
		componenttest.NewNopHost())

	innerCtx, _ = context.WithCancel(context.Background())

	envelopeStream, createErr = streamFactory.CreateStream(
		innerCtx,
		INVALID_SHARD_ID)

	require.Error(t, createErr)
	require.Nil(t, envelopeStream)
}