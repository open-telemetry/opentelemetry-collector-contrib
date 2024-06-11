// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/component/componenttest"
	"gotest.tools/assert"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.DeepEqual(t, &Config{
		HostIdentifiers:      []string{"host.id"},
		MetricsFlushInterval: 60 * time.Second,
	}, cfg)

	assert.NilError(t, componenttest.CheckConfigStruct(cfg))
}
