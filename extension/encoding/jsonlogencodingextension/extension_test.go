// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestExtension_Start_Shutdown(t *testing.T) {
	j := &jsonLogExtension{}
	err := j.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	err = j.Shutdown(context.Background())
	require.NoError(t, err)
}
