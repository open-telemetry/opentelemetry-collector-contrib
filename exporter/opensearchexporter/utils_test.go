// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetSuffixTime(t *testing.T) {
	defaultCfg := newDefaultConfig().(*Config)
	defaultCfg.LogstashFormat.Enabled = true
	defaultCfg.LogsIndex = "logs-generic-default"
	testTime := time.Date(2023, 12, 2, 10, 10, 10, 1, time.UTC)
	index := GenerateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.Equal(t, "logs-generic-default-2023.12.02", index)

	defaultCfg.LogsIndex = "logstash"
	defaultCfg.LogstashFormat.PrefixSeparator = "."
	otelLogsIndex := GenerateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	// assert.NoError(t, err)
	assert.Equal(t, "logstash.2023.12.02", otelLogsIndex)

	defaultCfg.LogstashFormat.DateFormat = "%Y-%m-%d"
	// newOtelLogsIndex, err := GenerateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	// assert.NoError(t, err)
	newOtelLogsIndex := GenerateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.Equal(t, "logstash.2023-12-02", newOtelLogsIndex)

	defaultCfg.LogstashFormat.DateFormat = "%d/%m/%Y"
	// newOtelLogsIndexWithSpecDataFormat, err := GenerateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	// assert.NoError(t, err)
	newOtelLogsIndexWithSpecDataFormat := GenerateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.Equal(t, "logstash.02/12/2023", newOtelLogsIndexWithSpecDataFormat)
}
