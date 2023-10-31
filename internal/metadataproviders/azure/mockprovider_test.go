// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockProvider_Metadata(t *testing.T) {
	p := MockProvider{}
	p.On("Metadata").Return(&ComputeMetadata{Name: "foo"}, nil)
	metadata, err := p.Metadata(context.Background())
	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, "foo", metadata.Name)
}
