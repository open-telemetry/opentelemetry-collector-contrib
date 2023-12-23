// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	p "go.opentelemetry.io/collector/processor"
	r "go.opentelemetry.io/collector/receiver"
)

func TestCreateReceiverConfig(t *testing.T) {
	f := r.NewFactory(
		"otlp",
		func() component.Config {
			return struct{}{}
		},
	)
	cfg, err := GetCfgInfo(f)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestCreateProcessorConfig(t *testing.T) {
	f := p.NewFactory(
		"filter",
		func() component.Config {
			return struct{}{}
		},
	)
	cfg, err := GetCfgInfo(f)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}
