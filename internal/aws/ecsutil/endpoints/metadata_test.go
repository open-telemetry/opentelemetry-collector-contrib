// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package endpoints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ecsPrefersLatestTME(t *testing.T) {
	t.Setenv(TaskMetadataEndpointV3EnvVar, "http://3")
	t.Setenv(TaskMetadataEndpointV4EnvVar, "http://4")

	tme, err := GetTMEFromEnv()
	require.NoError(t, err)
	require.NotNil(t, tme)
	assert.Equal(t, "4", tme.Host)
}
