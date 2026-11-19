// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	require.Equal(t, &Config{
		HostIdentifiers:      []string{"k8s.node.name", "host.id", "k8s.node.uid"},
		MetricsFlushInterval: 60 * time.Second,
	}, cfg)

	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}
