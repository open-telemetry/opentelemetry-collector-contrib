// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestBuildValid(t *testing.T) {
	f := NewFactory()
	cfg := f.NewDefaultConfig("test")
	op, err := f.CreateOperator(componenttest.NewNopTelemetrySettings(), cfg)
	require.NoError(t, err)
	require.IsType(t, &Output{}, op)
}

func TestBuildIvalid(t *testing.T) {
	f := NewFactory()
	cfg := f.NewDefaultConfig("test")
	_, err := f.CreateOperator(component.TelemetrySettings{}, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build context is missing a logger")
}

func TestProcess(t *testing.T) {
	f := NewFactory()
	cfg := f.NewDefaultConfig("test")
	op, err := f.CreateOperator(componenttest.NewNopTelemetrySettings(), cfg)
	require.NoError(t, err)

	entry := entry.New()
	result := op.Process(context.Background(), entry)
	require.Nil(t, result)
}
