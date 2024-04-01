// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/neuron"

const (
	NeuronCoreUtilization                       = "neuroncore_utilization_ratio"
	NeuronCoreMemoryUtilizationConstants        = "neuroncore_memory_usage_constants"
	NeuronCoreMemoryUtilizationModelCode        = "neuroncore_memory_usage_model_code"
	NeuronCoreMemoryUtilizationSharedScratchpad = "neuroncore_memory_usage_model_shared_scratchpad"
	NeuronCoreMemoryUtilizationRuntimeMemory    = "neuroncore_memory_usage_runtime_memory"
	NeuronCoreMemoryUtilizationTensors          = "neuroncore_memory_usage_tensors"
	NeuronDeviceHardwareEccEvents               = "neurondevice_hw_ecc_events_total"
	NeuronExecutionStatus                       = "execution_status_total"
	NeuronExecutionErrors                       = "execution_errors_total"
	NeuronRuntimeMemoryUsage                    = "neuron_runtime_memory_used_bytes"
	NeuronExecutionLatency                      = "execution_latency_seconds"
)
