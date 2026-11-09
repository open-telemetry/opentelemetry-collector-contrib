// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
)

func TestNewDecodeConfig(t *testing.T) {
	t.Run("defaults to empty metadata", func(t *testing.T) {
		config, err := NewDecodeConfig()
		require.NoError(t, err)
		assert.Nil(t, config.MessageMetadata.Get("x-tenant"))
	})

	t.Run("later metadata replaces earlier metadata", func(t *testing.T) {
		first := client.NewMetadata(map[string][]string{"first": {"a"}})
		second := client.NewMetadata(map[string][]string{"second": {"b"}})
		config, err := NewDecodeConfig(WithMessageMetadata(first), WithMessageMetadata(second))
		require.NoError(t, err)

		assert.Nil(t, config.MessageMetadata.Get("first"))
		assert.Equal(t, []string{"b"}, config.MessageMetadata.Get("second"))
	})

	t.Run("rejects nil options", func(t *testing.T) {
		_, err := NewDecodeConfig(nil)
		require.ErrorContains(t, err, "nil")
	})

	t.Run("propagates validation errors and stops applying options", func(t *testing.T) {
		metadata := client.NewMetadata(map[string][]string{"x-tenant": {"tenant-a"}})
		config, err := NewDecodeConfig(
			WithMessageMetadata(metadata),
			messageDecodeOptionFunc(func(*DecodeConfig) error { return assert.AnError }),
			messageDecodeOptionFunc(func(*DecodeConfig) error {
				t.Error("applied an option after validation failed")
				return nil
			}),
		)
		require.ErrorIs(t, err, assert.AnError)
		assert.Nil(t, config.MessageMetadata.Get("x-tenant"))
	})
}

type messageDecodeOptionFunc func(*DecodeConfig) error

func (f messageDecodeOptionFunc) apply(config *DecodeConfig) error {
	return f(config)
}
