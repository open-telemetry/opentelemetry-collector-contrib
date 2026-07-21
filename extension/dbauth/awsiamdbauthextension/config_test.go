// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	require.NoError(t, (&Config{Region: "us-east-1"}).Validate(), "a region is the only required field")
	require.ErrorIs(t, (&Config{}).Validate(), errNoRegion, "an empty region fails at config load")
	assert.NoError(t, (&Config{Region: "us-east-1", Endpoint: "db:5432", DBUser: "monitor"}).Validate(),
		"optional endpoint/db_user are allowed alongside a region")
}
