// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestDefaultConfig_exporterSettings(t *testing.T) {
	want := &Config{
		RetrySettings:      exporterhelper.NewDefaultRetrySettings(),
		AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
		QueueSettings: QueueSettings{
			QueueSize: exporterhelper.NewDefaultQueueSettings().QueueSize,
		},
	}
	assert.Equal(t, want, createDefaultConfig())
}
