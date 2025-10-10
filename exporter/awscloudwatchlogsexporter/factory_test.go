// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestDefaultConfig_exporterSettings(t *testing.T) {
	want := &Config{
		BackOffConfig:      configretry.NewDefaultBackOffConfig(),
		AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
		QueueSettings: func() exporterhelper.QueueBatchConfig {
			queue := exporterhelper.NewDefaultQueueConfig()
			queue.NumConsumers = 1
			return queue
		}(),
	}
	assert.Equal(t, want, createDefaultConfig())
}
