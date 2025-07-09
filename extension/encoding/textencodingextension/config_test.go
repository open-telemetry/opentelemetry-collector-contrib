// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DefaultConfig(t *testing.T) {
	c := createDefaultConfig().(*Config)
	require.NoError(t, c.Validate())
}

func Test_ConfigValidate_Encoding(t *testing.T) {
	c := &Config{Encoding: "bbq"}
	require.Error(t, c.Validate())
}

func Test_ConfigValidate_Unmarshaler(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.UnmarshalingSeparator = `??\`
	require.Error(t, c.Validate())
}
