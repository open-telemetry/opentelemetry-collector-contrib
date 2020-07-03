// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cudareceiver

/*
#cgo CFLAGS: -I/usr/local/cuda-10.2/include/
#cgo LDFLAGS: -L/usr/lib -L/usr/local/cuda-10.2/targets/x86_64-linux/lib/stubs/ -lnvidia-ml
#include <stdio.h>
#include <nvml.h>
*/
import "C"

// NVMLReturn is the wrapper of nvmlReturn_t
// https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1g06fa9b5de08c6cc716fbf565e93dd3d0
type NVMLReturn int

const (
	Success                  NVMLReturn = NVMLReturn(C.NVML_SUCCESS)
	ErrUninitialized                    = NVMLReturn(C.NVML_ERROR_UNINITIALIZED)
	ErrInvalidArgument                  = NVMLReturn(C.NVML_ERROR_INVALID_ARGUMENT)
	ErrNotSupported                     = NVMLReturn(C.NVML_ERROR_NOT_SUPPORTED)
	ErrNoPermission                     = NVMLReturn(C.NVML_ERROR_NO_PERMISSION)
	ErrAlreadyInitialized               = NVMLReturn(C.NVML_ERROR_ALREADY_INITIALIZED)
	ErrNotFound                         = NVMLReturn(C.NVML_ERROR_NOT_FOUND)
	ErrInsufficientSize                 = NVMLReturn(C.NVML_ERROR_INSUFFICIENT_SIZE)
	ErrInsufficientPower                = NVMLReturn(C.NVML_ERROR_INSUFFICIENT_POWER)
	ErrDriverNotLoaded                  = NVMLReturn(C.NVML_ERROR_DRIVER_NOT_LOADED)
	ErrTimeout                          = NVMLReturn(C.NVML_ERROR_TIMEOUT)
	ErrIRQIssue                         = NVMLReturn(C.NVML_ERROR_IRQ_ISSUE)
	ErrLibraryNotFound                  = NVMLReturn(C.NVML_ERROR_LIBRARY_NOT_FOUND)
	ErrFunctionNotFound                 = NVMLReturn(C.NVML_ERROR_FUNCTION_NOT_FOUND)
	ErrCorruputedInfoROM                = NVMLReturn(C.NVML_ERROR_CORRUPTED_INFOROM)
	ErrGPUIsLost                        = NVMLReturn(C.NVML_ERROR_GPU_IS_LOST)
	ErrResetRequired                    = NVMLReturn(C.NVML_ERROR_RESET_REQUIRED)
	ErrOperatingSystem                  = NVMLReturn(C.NVML_ERROR_OPERATING_SYSTEM)
	ErrLibRMVersionMismatch             = NVMLReturn(C.NVML_ERROR_LIB_RM_VERSION_MISMATCH)
	ErrInUse                            = NVMLReturn(C.NVML_ERROR_IN_USE)
	ErrMemory                           = NVMLReturn(C.NVML_ERROR_MEMORY)
	ErrNoData                           = NVMLReturn(C.NVML_ERROR_NO_DATA)
	ErrVGPUECCNotSupported              = NVMLReturn(C.NVML_ERROR_VGPU_ECC_NOT_SUPPORTED)
	// ErrInsufficientResources            = int(C.NVML_ERROR_INSUFFICIENT_RESOURCES)
	ErrUnknown                          = NVMLReturn(C.NVML_ERROR_UNKNOWN)
)

// NVMLPCIeUtilCounter is the wrapper of nvmlPcieUtilCounter_t
// https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceStructs.html#group__nvmlDeviceStructs_1g0840d5d019555b267b45609c5568ced5
type NVMLPCIeUtilCounter int

const (
	PCIeUtilTXBytes NVMLPCIeUtilCounter = NVMLPCIeUtilCounter(C.NVML_PCIE_UTIL_TX_BYTES)
	PCIeUtilRxBytes                     = NVMLPCIeUtilCounter(C.NVML_PCIE_UTIL_RX_BYTES)
	PCIeUtilCount                       = NVMLPCIeUtilCounter(C.NVML_PCIE_UTIL_COUNT)
)

// NVMLDevice is the wrapper of nvmlDevice_t
type NVMLDevice struct {
	device C.nvmlDevice_t
}

// NVMLInit is the wrapper of nvmlInit()
func NVMLInit() NVMLReturn {
	r := C.nvmlInit()
	return NVMLReturn(r)
}

// NVMLShutdown is the wrapper of nvmlShutdown()
func NVMLShutdown() NVMLReturn {
	r := C.nvmlShutdown()
	return NVMLReturn(r)
}

// NVMLDeviceGetHandledByIndex is the wrapper of nvmlDeviceGetHandleByIndex()
func NVMLDeviceGetHandledByIndex(gpuID uint64) (*NVMLDevice, NVMLReturn) {
	var device C.nvmlDevice_t
	r := C.nvmlDeviceGetHandleByIndex(C.uint(gpuID), &device)
	return &NVMLDevice{device: device}, NVMLReturn(r)
}

// Temperature is the wrapper of nvmlDeviceGetTemperature
func (d *NVMLDevice) Temperature() uint64 {
	var temp C.uint
	C.nvmlDeviceGetTemperature(d.device, C.NVML_TEMPERATURE_GPU, &temp)
	return uint64(temp)
}

// PowerUsage is the wrapper of nvmlDeviceGetPowerUsage
func (d *NVMLDevice) PowerUsage() uint64 {
	var power C.uint
	C.nvmlDeviceGetPowerUsage(d.device, &power)
	return uint64(power)
}

// PCIeThroughput is the wrapper of nvmlDeviceGetPcieThroughput
func (d *NVMLDevice) PCIeThroughput(c NVMLPCIeUtilCounter) uint64 {
	var value C.uint
	counter := C.nvmlPcieUtilCounter_t(c)
	C.nvmlDeviceGetPcieThroughput(d.device, counter, &value)
	return uint64(value)
}
