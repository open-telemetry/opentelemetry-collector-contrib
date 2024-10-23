// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := NewDefaultConfig()
	assert.NotNil(t, cfg)
	assert.Nil(t, cfg.Validate())
}
