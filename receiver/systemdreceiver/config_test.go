// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Config_Validate(t *testing.T) {
	c := Config{}
	require.Error(t, c.Validate())

	c = Config{
		Units: []string{"foo.unit"},
	}

	require.NoError(t, c.Validate())
}
