// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package avrologencodingextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	assert.ErrorIs(t, err, errNoSchema)

	cfg.Schema = "schema1"
	err = cfg.Validate()
	assert.NoError(t, err)
}
