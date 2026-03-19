// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

type mockBuilder struct{}

func (*mockBuilder) Build(_ component.TelemetrySettings) (Operator, error) {
	return nil, nil
}

func (*mockBuilder) ID() string     { return "" }
func (*mockBuilder) Type() string   { return "" }
func (*mockBuilder) SetID(_ string) {}

func TestRegistryRegisterAndLookup(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	require.NotNil(t, reg)

	builderFunc := func() Builder {
		return &mockBuilder{}
	}

	reg.Register("mock_operator", builderFunc)

	b, ok := reg.Lookup("mock_operator")
	require.True(t, ok)
	require.NotNil(t, b)

	b2, ok2 := reg.Lookup("non_existent")
	require.False(t, ok2)
	require.Nil(t, b2)
}

func TestGlobalRegistryRegisterAndLookup(t *testing.T) {
	t.Parallel()
	builderFunc := func() Builder {
		return &mockBuilder{}
	}

	Register("global_mock_operator", builderFunc)

	b, ok := Lookup("global_mock_operator")
	require.True(t, ok)
	require.NotNil(t, b)

	b2, ok2 := Lookup("global_non_existent")
	require.False(t, ok2)
	require.Nil(t, b2)
}
