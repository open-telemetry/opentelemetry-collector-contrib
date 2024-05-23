// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_Validate_Invalid(t *testing.T) {
	cfg := Config{
		Metadata: []string{
			"geo.city",
		},
	}
	assert.Error(t, cfg.Validate())
}

func TestLoadConfig_Validate_Valid(t *testing.T) {
	cfg := Config{}
	assert.NoError(t, cfg.Validate())
}
