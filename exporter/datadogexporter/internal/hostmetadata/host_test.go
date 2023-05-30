// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestHost(t *testing.T) {
	p, err := GetSourceProvider(componenttest.NewNopTelemetrySettings(), "test-host")
	require.NoError(t, err)
	src, err := p.Source(context.Background())
	require.NoError(t, err)
	assert.Equal(t, src.Identifier, "test-host")
}
