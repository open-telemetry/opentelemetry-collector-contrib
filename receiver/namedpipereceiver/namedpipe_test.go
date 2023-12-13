// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package namedpipereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/namedpipereceiver"

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewID("namedpipe").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.NoError(t, component.ValidateConfig(cfg))
	assert.Equal(t, testdataConfigYaml(), cfg)
}

func TestReadPipe(t *testing.T) {
	t.Parallel()

	f := NewFactory()
	sink := new(consumertest.LogsSink)
	cfg := testdataConfigYaml()

	converter := adapter.NewConverter(zap.NewNop())
	converter.Start()
	defer converter.Stop()

	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	pipe, err := os.OpenFile("/tmp/pipe", os.O_WRONLY, 0o600)
	require.NoError(t, err, "failed to open pipe")
	defer pipe.Close()

	// Write 10 logs into the pipe and assert that they all come out the other end.
	numLogs := 10
	for i := 0; i < numLogs; i++ {
		_, err = pipe.WriteString("test\n")
		require.NoError(t, err, "failed to write to pipe")
	}

	assert.Eventually(t, func() bool {
		return sink.LogRecordCount() == numLogs
	}, time.Second, 100*time.Millisecond)
}

func testdataConfigYaml() *NamedPipeConfig {
	return &NamedPipeConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
			RetryOnFailure: consumerretry.Config{
				Enabled:         false,
				InitialInterval: 1 * time.Second,
				MaxInterval:     30 * time.Second,
				MaxElapsedTime:  5 * time.Minute,
			},
		},
		InputConfig: func() namedpipe.Config {
			c := namedpipe.NewConfig()
			c.Path = "/tmp/pipe"
			c.Permissions = 0o600
			c.Encoding = "utf-8"

			return *c
		}(),
	}
}
