// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectorydsreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	t.Parallel()

	fact := NewFactory()
	require.NotNil(t, fact)
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig()
	require.NotNil(t, conf)
}
