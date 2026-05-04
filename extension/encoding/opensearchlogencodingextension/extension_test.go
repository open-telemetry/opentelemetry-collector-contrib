// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchlogencodingextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestExtension_Start_Shutdown(t *testing.T) {
	e := &opensearchLogExtension{}
	err := e.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	err = e.Shutdown(t.Context())
	require.NoError(t, err)
}
