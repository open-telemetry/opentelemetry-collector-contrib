// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package foundationdbreceiver

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[component.NewID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), r0)

	r1 := cfg.Receivers[component.NewIDWithName(typeStr, "receiver_settings")]
	assert.Equal(t, &Config{
		ReceiverSettings: receiverhelper.NewReceiverSettings(component.NewIDWithName(typeStr, "receiver_settings")),
		Address:          "localhost:8889",
		MaxPacketSize:    defaultMaxPacketSize,
		SocketBufferSize: defaultSocketBufferSize,
		Format:           "opentelemetry",
	}, r1)
}

func TestValidateConfig(t *testing.T) {
	type test struct {
		name   string
		config *Config
		error  error
	}

	tests := []test{
		{
			name: "maxPacketSizeTooSmall",
			config: &Config{
				MaxPacketSize: -100,
				Format:        "opentelemetry",
			},
			error: maxPacketSizeErr,
		},
		{
			name: "maxPacketSizeTooLarge",
			config: &Config{
				MaxPacketSize: 65536,
				Format:        "opentelemetry",
			},
			error: maxPacketSizeErr,
		},
		{
			name: "socketBufferSizeToSmall",
			config: &Config{
				MaxPacketSize:    defaultMaxPacketSize,
				Format:           "opentelemetry",
				SocketBufferSize: -1,
			},
			error: socketBufferSizeErr,
		},
		{
			name: "improperAddress",
			config: &Config{
				MaxPacketSize: defaultMaxPacketSize,
				Address:       "foo",
				Format:        "opentelemetry",
			},
			error: fmt.Errorf("endpoint is not formatted correctly: address foo: missing port in address"),
		},
		{
			name: "improperNANPortAddress",
			config: &Config{
				MaxPacketSize: defaultMaxPacketSize,
				Address:       "foo:xyx",
				Format:        "opentelemetry",
			},
			error: fmt.Errorf("endpoint port is not a number: strconv.ParseInt: parsing \"xyx\": invalid syntax"),
		},
		{
			name: "illegalPortAddress",
			config: &Config{
				MaxPacketSize: defaultMaxPacketSize,
				Address:       "foo:70000",
				Format:        "opentelemetry",
			},
			error: portNumberRangeErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, test.config.validate(), test.error.Error())
		})
	}
}
