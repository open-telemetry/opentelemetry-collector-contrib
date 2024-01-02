// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInProcessPipeline(t *testing.T) {
	factories, err := Components()
	assert.NoError(t, err)
	sender := NewOTLPTraceDataSender(DefaultHost, GetAvailablePort(t))
	receiver := NewOTLPDataReceiver(DefaultOTLPPort)
	runner, ok := NewInProcessCollector(factories).(*inProcessCollector)
	require.True(t, ok)

	format := `
receivers:%v
exporters:%v
processors:
  batch:

extensions:

service:
  extensions:
  pipelines:
    traces:
      receivers: [%v]
      processors: [batch]
      exporters: [%v]
`
	config := fmt.Sprintf(
		format,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		sender.ProtocolName(),
		receiver.ProtocolName(),
	)
	configCleanup, cfgErr := runner.PrepareConfig(config)
	defer configCleanup()
	assert.NoError(t, cfgErr)
	assert.NotNil(t, configCleanup)
	assert.NotEmpty(t, runner.configStr)
	args := StartParams{}
	defer func() {
		_, err := runner.Stop()
		require.NoError(t, err)
	}()
	assert.NoError(t, runner.Start(args))
	assert.NotNil(t, runner.svc)
}
