// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestSanitize(t *testing.T) {

	defer testutil.SetFeatureGateForTest(t, dropSanitizationGate, false)()

	require.Equal(t, "", NormalizeLabel(""), "")
	require.Equal(t, "key_test", NormalizeLabel("_test"))
	require.Equal(t, "key_0test", NormalizeLabel("0test"))
	require.Equal(t, "test", NormalizeLabel("test"))
	require.Equal(t, "test__", NormalizeLabel("test_/"))
	require.Equal(t, "__test", NormalizeLabel("__test"))
}

func TestSanitizeDropSanitization(t *testing.T) {

	defer testutil.SetFeatureGateForTest(t, dropSanitizationGate, true)()

	require.Equal(t, "", NormalizeLabel(""))
	require.Equal(t, "_test", NormalizeLabel("_test"))
	require.Equal(t, "key_0test", NormalizeLabel("0test"))
	require.Equal(t, "test", NormalizeLabel("test"))
	require.Equal(t, "__test", NormalizeLabel("__test"))
}

func TestAlreadyRegisteredDropSanitizationGate(t *testing.T) {

	// Register() should return an error since it's already registered
	var _, err = featuregate.GlobalRegistry().Register(
		dropSanitizationGate.ID(),
		dropSanitizationGate.Stage(),
		featuregate.WithRegisterDescription(dropSanitizationGate.Description()),
		featuregate.WithRegisterReferenceURL(dropSanitizationGate.ReferenceURL()),
	)
	require.NotNil(t, err)

	// mustRegisterOrLoadGate() should return the already registered gate
	var gate = mustRegisterOrLoadGate(
		dropSanitizationGate.ID(),
		dropSanitizationGate.Stage(),
		featuregate.WithRegisterDescription(dropSanitizationGate.Description()),
		featuregate.WithRegisterReferenceURL(dropSanitizationGate.ReferenceURL()),
	)
	require.NotNil(t, gate)
	require.Equal(t, gate.ID(), dropSanitizationGate.ID())
	require.Equal(t, gate.Stage(), dropSanitizationGate.Stage())
	require.Equal(t, gate.Description(), dropSanitizationGate.Description())
	require.Equal(t, gate.ReferenceURL(), dropSanitizationGate.ReferenceURL())
	require.Equal(t, gate, dropSanitizationGate)
}
