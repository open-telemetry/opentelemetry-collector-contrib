// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata

import (
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestHost(t *testing.T) {
	p, err := GetSourceProvider(componenttest.NewNopTelemetrySettings(), "test-host", 31*time.Second)
	require.NoError(t, err)
	src, err := p.Source(t.Context())
	require.NoError(t, err)
	assert.Equal(t, source.Source{Kind: source.HostnameKind, Identifier: "test-host"}, src)
}
