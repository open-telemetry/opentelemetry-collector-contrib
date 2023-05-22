// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerobserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestValidConfig(t *testing.T) {
	err := componenttest.CheckConfigStruct(createDefaultConfig())
	require.NoError(t, err)
}

func TestCreateExtension(t *testing.T) {
	dockerObserver, err := createExtension(
		context.Background(),
		extensiontest.NewNopCreateSettings(),
		&Config{},
	)
	require.NoError(t, err)
	require.NotNil(t, dockerObserver)
}
