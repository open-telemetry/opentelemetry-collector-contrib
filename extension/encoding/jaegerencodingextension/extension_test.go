// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestExtension_Start(t *testing.T) {
	j := &jaegerExtension{
		config: createDefaultConfig().(*Config),
	}
	err := j.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
}
