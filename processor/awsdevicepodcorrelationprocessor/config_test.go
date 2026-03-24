// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsdevicepodcorrelationprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "neuron", DeviceIDAttribute: "NeuronDevice", ResourceNames: []string{"aws.amazon.com/neurondevice"}},
		},
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_EmptyDeviceTypes(t *testing.T) {
	cfg := &Config{}
	assert.ErrorContains(t, cfg.Validate(), "device_types must not be empty")
}

func TestValidate_MissingName(t *testing.T) {
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{DeviceIDAttribute: "dev", ResourceNames: []string{"res"}},
		},
	}
	assert.ErrorContains(t, cfg.Validate(), "name must not be empty")
}

func TestValidate_MissingDeviceIDAttribute(t *testing.T) {
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", ResourceNames: []string{"res"}},
		},
	}
	assert.ErrorContains(t, cfg.Validate(), "device_id_attribute must not be empty")
}

func TestValidate_MissingResourceNames(t *testing.T) {
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev"},
		},
	}
	assert.ErrorContains(t, cfg.Validate(), "resource_names must not be empty")
}

func TestValidate_InvalidDeviceIDSource(t *testing.T) {
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev", DeviceIDSource: "invalid", ResourceNames: []string{"res"}},
		},
	}
	assert.ErrorContains(t, cfg.Validate(), "device_id_source must be")
}

func TestValidate_DuplicateName(t *testing.T) {
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev1", ResourceNames: []string{"res1"}},
			{Name: "gpu", DeviceIDAttribute: "dev2", ResourceNames: []string{"res2"}},
		},
	}
	assert.ErrorContains(t, cfg.Validate(), "duplicate name")
}

func TestValidate_DefaultsDeviceIDSource(t *testing.T) {
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev", ResourceNames: []string{"res"}},
		},
	}
	require.NoError(t, cfg.Validate())
	assert.Empty(t, cfg.DeviceTypes[0].DeviceIDSource)

	cfg.setDefaults()
	assert.Equal(t, DeviceIDSourceDatapoint, cfg.DeviceTypes[0].DeviceIDSource)
}
