// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())
	cfg := f.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	duration, _ := time.ParseDuration("30s")
	assert.Equal(t, duration, cfg.(*Config).CollectionInterval)
}
