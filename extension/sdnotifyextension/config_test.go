// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdnotifyextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestValidate(t *testing.T) {
	cfg := &Config{}
	assert.NoError(t, cfg.Validate())
	assert.NoError(t, confmap.Validate(cfg))
}
