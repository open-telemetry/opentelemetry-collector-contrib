// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func Test_createDefaultConfig(t *testing.T) {
	expectedCfg := &Config{
		API:                   chronicleAPI,
		Hostname:              defaultHostname,
		APIVersion:            apiVersionV1Alpha,
		TimeoutConfig:         exporterhelper.NewDefaultTimeoutConfig(),
		QueueBatchConfig:      configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		BackOffConfig:         configretry.NewDefaultBackOffConfig(),
		Compression:           noCompression,
		CollectAgentMetrics:   true,
		MetricsInterval:       defaultMetricsInterval,
		BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
		CollectorID:           defaultCollectorID,
	}

	actual := createDefaultConfig()
	require.Equal(t, expectedCfg, actual)
}
