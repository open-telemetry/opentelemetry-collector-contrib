// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package key_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"
)

func TestEnsureDifferentKeys(t *testing.T) {
	t.Parallel()

	k := key.Randomized(nil)
	assert.NotEmpty(t, k, "Must have a string that has a value")
	assert.NotEqual(t, k, key.Randomized(nil), "Must have different string values")
}
