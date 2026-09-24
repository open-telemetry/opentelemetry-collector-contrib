// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/render/internal/metadata"
)

func TestDetectRenderEnv(t *testing.T) {
	t.Setenv("RENDER_SERVICE_ID", "srv-abc123")
	t.Setenv("RENDER_SERVICE_NAME", "my-service")
	t.Setenv("RENDER_INSTANCE_ID", "inst-xyz")
	t.Setenv("RENDER_GIT_COMMIT", "deadbeef")

	d := &detector{logger: zap.NewNop(), rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	res, schemaURL, err := d.Detect(context.Background())

	assert.NoError(t, err)
	assert.NotEmpty(t, schemaURL)
	v, ok := res.Attributes().Get("service.name")
	assert.True(t, ok)
	assert.Equal(t, "my-service", v.AsString())
}

func TestDetectRenderEnvMissing(t *testing.T) {
	d := &detector{logger: zap.NewNop(), rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	res, _, err := d.Detect(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
}
