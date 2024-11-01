// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter

import (
	"context"
	"testing"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

func TestGenerateClientOptions(t *testing.T) {
	factory := NewFactory()

	t.Run("defaults", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.ProjectID = "my-project"
		cfg.Topic = "projects/my-project/topics/otlp"

		require.NoError(t, cfg.Validate())

		gotOptions, closeConnFn, err := generateClientOptions(cfg, "test-user-agent 6789")
		assert.NoError(t, err)
		assert.Empty(t, closeConnFn)

		expectedOptions := []option.ClientOption{
			option.WithUserAgent("test-user-agent 6789"),
		}
		assert.ElementsMatch(t, expectedOptions, gotOptions)
	})

	t.Run("secure custom endpoint", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.ProjectID = "my-project"
		cfg.Topic = "projects/my-project/topics/otlp"
		cfg.Endpoint = "defg"

		require.NoError(t, cfg.Validate())

		gotOptions, closeConnFn, err := generateClientOptions(cfg, "test-user-agent 4321")
		assert.NoError(t, err)
		assert.Empty(t, closeConnFn)

		expectedOptions := []option.ClientOption{
			option.WithUserAgent("test-user-agent 4321"),
			option.WithEndpoint("defg"),
		}
		assert.ElementsMatch(t, expectedOptions, gotOptions)
	})

	t.Run("insecure endpoint", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.ProjectID = "my-project"
		cfg.Topic = "projects/my-project/topics/otlp"
		cfg.Endpoint = "abcd"
		cfg.Insecure = true

		require.NoError(t, cfg.Validate())

		gotOptions, closeConnFn, err := generateClientOptions(cfg, "test-user-agent 1234")
		assert.NoError(t, err)
		assert.NotEmpty(t, closeConnFn)
		assert.NoError(t, closeConnFn())

		require.Len(t, gotOptions, 2)
		assert.Equal(t, option.WithUserAgent("test-user-agent 1234"), gotOptions[0])
		assert.IsType(t, option.WithGRPCConn(nil), gotOptions[1])
	})
}

func TestNewPublisherClient(t *testing.T) {
	// The publisher client checks for credentials during init
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/gcp-fake-creds.json")

	ctx := context.Background()
	factory := NewFactory()

	t.Run("defaults", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.ProjectID = "my-project"
		cfg.Topic = "projects/my-project/topics/otlp"

		require.NoError(t, cfg.Validate())

		client, err := newPublisherClient(ctx, cfg, "test-user-agent 6789")
		assert.NoError(t, err)
		require.NotEmpty(t, client)
		assert.IsType(t, &pubsub.PublisherClient{}, client)
		assert.NoError(t, client.Close())
	})

	t.Run("secure custom endpoint", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.ProjectID = "my-project"
		cfg.Topic = "projects/my-project/topics/otlp"
		cfg.Endpoint = "xyz"

		require.NoError(t, cfg.Validate())

		client, err := newPublisherClient(ctx, cfg, "test-user-agent 6789")
		assert.NoError(t, err)
		require.NotEmpty(t, client)
		assert.IsType(t, &pubsub.PublisherClient{}, client)
		assert.NoError(t, client.Close())
	})

	t.Run("insecure endpoint", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.ProjectID = "my-project"
		cfg.Topic = "projects/my-project/topics/otlp"
		cfg.Endpoint = "abc"
		cfg.Insecure = true

		require.NoError(t, cfg.Validate())

		client, err := newPublisherClient(ctx, cfg, "test-user-agent 6789")
		assert.NoError(t, err)
		require.NotEmpty(t, client)
		assert.IsType(t, &wrappedPublisherClient{}, client)
		assert.NoError(t, client.Close())
	})
}
