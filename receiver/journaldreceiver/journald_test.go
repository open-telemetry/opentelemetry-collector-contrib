// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package journaldreceiver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, testdataConfigYaml(), cfg)
}

func TestInputConfigFailure(t *testing.T) {
	sink := new(consumertest.LogsSink)
	factory := NewFactory()
	badCfg := &JournaldConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: func() journald.Config {
			c := journald.NewConfig()
			c.StartAt = "middle"
			return *c
		}(),
	}
	receiver, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), badCfg, sink)
	require.Error(t, err, "receiver creation should fail if input config isn't valid")
	require.Nil(t, receiver, "receiver creation should fail if input config isn't valid")
}

func testdataConfigYaml() *JournaldConfig {
	return &JournaldConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: func() journald.Config {
			c := journald.NewConfig()
			c.Units = []string{"ssh"}
			c.Priority = "info"
			dir := "/run/log/journal"
			c.Directory = &dir
			return *c
		}(),
	}
}
