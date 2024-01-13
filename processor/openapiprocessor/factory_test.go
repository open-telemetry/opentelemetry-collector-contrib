// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {

	cfg := createDefaultConfig()

	assert.Equal(t,
		cfg,
		&Config{
			APILoadTimeout:    defaultTimeout,
			APIReloadInterval: defaultReloadInterval,
			AllowHTTPAndHTTPS: false,
		},
	)
}
