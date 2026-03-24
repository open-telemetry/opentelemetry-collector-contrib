// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsdevicepodcorrelationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsdevicepodcorrelationprocessor"

import (
	"errors"
	"fmt"
)

// Config defines the configuration for the awsdevicepodcorrelation processor.
type Config struct {
	// KubeletSocketPath is the path to the Kubelet Pod Resources API socket.
	// Defaults to "/var/lib/kubelet/pod-resources/kubelet.sock".
	KubeletSocketPath string `mapstructure:"kubelet_socket_path"`

	// DeviceTypes is a list of device type configurations, each defining how to
	// correlate one class of devices (e.g., neuron, EFA, GPU) with pod metadata.
	DeviceTypes []DeviceTypeConfig `mapstructure:"device_types"`
}

// DeviceIDSource indicates where the device ID attribute lives in the OTEL data model.
type DeviceIDSource string

const (
	// DeviceIDSourceDatapoint means the device ID is a datapoint-level attribute.
	DeviceIDSourceDatapoint DeviceIDSource = "datapoint"
	// DeviceIDSourceResource means the device ID is a resource-level attribute.
	DeviceIDSourceResource DeviceIDSource = "resource"
)

// DeviceTypeConfig defines the configuration for a single device type.
type DeviceTypeConfig struct {
	// Name uniquely identifies this device type (e.g., "neuron", "efa", "gpu").
	Name string `mapstructure:"name"`

	// DeviceIDAttribute is the metric attribute key that holds the device identifier
	// used for PodResourcesStore lookup (e.g., "NeuronDevice", "device").
	DeviceIDAttribute string `mapstructure:"device_id_attribute"`

	// DeviceIDSource indicates whether device_id_attribute is found on the
	// datapoint ("datapoint") or on the resource ("resource"). Defaults to "datapoint".
	DeviceIDSource DeviceIDSource `mapstructure:"device_id_source"`

	// ResourceNames is an ordered list of Kubernetes extended resource names to try
	// during PodResourcesStore lookup (e.g., ["aws.amazon.com/neurondevice", "aws.amazon.com/neuron"]).
	// The processor tries each in order until a match is found.
	ResourceNames []string `mapstructure:"resource_names"`
}

// setDefaults applies default values to the configuration.
func (cfg *Config) setDefaults() {
	for i := range cfg.DeviceTypes {
		if cfg.DeviceTypes[i].DeviceIDSource == "" {
			cfg.DeviceTypes[i].DeviceIDSource = DeviceIDSourceDatapoint
		}
	}
}

// Validate checks if the processor configuration is valid.
func (cfg *Config) Validate() error {
	if len(cfg.DeviceTypes) == 0 {
		return errors.New("device_types must not be empty")
	}

	seen := make(map[string]bool, len(cfg.DeviceTypes))
	for i := range cfg.DeviceTypes {
		dt := &cfg.DeviceTypes[i]
		if dt.Name == "" {
			return fmt.Errorf("device_types[%d]: name must not be empty", i)
		}
		if dt.DeviceIDAttribute == "" {
			return fmt.Errorf("device_types[%d]: device_id_attribute must not be empty", i)
		}
		if len(dt.ResourceNames) == 0 {
			return fmt.Errorf("device_types[%d]: resource_names must not be empty", i)
		}
		if dt.DeviceIDSource != "" && dt.DeviceIDSource != DeviceIDSourceDatapoint && dt.DeviceIDSource != DeviceIDSourceResource {
			return fmt.Errorf("device_types[%d]: device_id_source must be %q or %q, got %q", i, DeviceIDSourceDatapoint, DeviceIDSourceResource, dt.DeviceIDSource)
		}
		if seen[dt.Name] {
			return fmt.Errorf("device_types[%d]: duplicate name %q", i, dt.Name)
		}
		seen[dt.Name] = true
	}

	return nil
}
