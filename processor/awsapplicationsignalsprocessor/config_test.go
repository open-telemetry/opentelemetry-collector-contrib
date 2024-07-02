// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsapplicationsignalsprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePassed(t *testing.T) {
	config := Config{}
	assert.Nil(t, config.Validate())
}
