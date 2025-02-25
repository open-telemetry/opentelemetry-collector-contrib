// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestHost(t *testing.T) {
	p, err := GetSourceProvider(componenttest.NewNopTelemetrySettings(), "test-host", 31*time.Second)
	require.NoError(t, err)
	src, err := p.Source(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-host", src.Identifier)
}
