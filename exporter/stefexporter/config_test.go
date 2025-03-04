// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, confmap.New().Unmarshal(&cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
}
