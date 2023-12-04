// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestExtension_Start(t *testing.T) {
	j := &zipkinExtension{
		config: createDefaultConfig().(*Config),
	}
	err := j.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
}

func TestExtension_Err(t *testing.T) {
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Protocol = "v3"
	_, err := factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NotNil(t, err)
	require.ErrorContains(t, err, "unsupported protocol: \"v3\"")
}
