// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := createTypedDefaultConfig()
	assert.NotNil(t, cfg)
	assert.NoError(t, cfg.Validate())
}
