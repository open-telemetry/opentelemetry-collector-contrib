// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	cfg := createDefaultConfig()
	rc := cfg.(*Config)
	require.Error(t, rc.Validate())

	rc.Queries = []string{"select * from certificates"}
	assert.NoError(t, rc.Validate())
}
