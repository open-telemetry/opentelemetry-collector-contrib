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
#include <nvml.h>
*/
import "C"

// TODO: Add build flags for the locations of header files and dynamic libraries.

// NVMLReturn is the wrapper of nvmlReturn_t
// https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1g06fa9b5de08c6cc716fbf565e93dd3d0
type NVMLReturn int

const (
	NVMLSuccess                 NVMLReturn = NVMLReturn(C.NVML_SUCCESS)
	NVMLErrUninitialized                   = NVMLReturn(C.NVML_ERROR_UNINITIALIZED)
	NVMLErrInvalidArgument                 = NVMLReturn(C.NVML_ERROR_INVALID_ARGUMENT)
	NVMLErrNotSupported                    = NVMLReturn(C.NVML_ERROR_NOT_SUPPORTED)
	NVMLErrNoPermission                    = NVMLReturn(C.NVML_ERROR_NO_PERMISSION)
	NVMLErrAlreadyInitialized              = NVMLReturn(C.NVML_ERROR_ALREADY_INITIALIZED)
	NVMLErrNotFound                        = NVMLReturn(C.NVML_ERROR_NOT_FOUND)
	NVMLErrInsufficientSize                = NVMLReturn(C.NVML_ERROR_INSUFFICIENT_SIZE)
	NVMLErrInsufficientPower               = NVMLReturn(C.NVML_ERROR_INSUFFICIENT_POWER)
	NVMLErrDriverNotLoaded                 = NVMLReturn(C.NVML_ERROR_DRIVER_NOT_LOADED)
	NVMLErrTimeout                         = NVMLReturn(C.NVML_ERROR_TIMEOUT)
	NVMLErrIRQIssue                        = NVMLReturn(C.NVML_ERROR_IRQ_ISSUE)
	NVMLErrLibraryNotFound                 = NVMLReturn(C.NVML_ERROR_LIBRARY_NOT_FOUND)
	NVMLErrFunctionNotFound                = NVMLReturn(C.NVML_ERROR_FUNCTION_NOT_FOUND)
	NVMLErrCorruputedInfoROM               = NVMLReturn(C.NVML_ERROR_CORRUPTED_INFOROM)
	NVMLErrGPUIsLost                       = NVMLReturn(C.NVML_ERROR_GPU_IS_LOST)
	NVMLErrResetRequired                   = NVMLReturn(C.NVML_ERROR_RESET_REQUIRED)
	NVMLErrOperatingSystem                 = NVMLReturn(C.NVML_ERROR_OPERATING_SYSTEM)
	NVMLErrLibRMVersionMismatch            = NVMLReturn(C.NVML_ERROR_LIB_RM_VERSION_MISMATCH)
	NVMLErrInUse                           = NVMLReturn(C.NVML_ERROR_IN_USE)
	NVMLErrMemory                          = NVMLReturn(C.NVML_ERROR_MEMORY)
	NVMLErrNoData                          = NVMLReturn(C.NVML_ERROR_NO_DATA)
	NVMLErrVGPUECCNotSupported             = NVMLReturn(C.NVML_ERROR_VGPU_ECC_NOT_SUPPORTED)
	// ErrInsufficientResources            = int(C.NVML_ERROR_INSUFFICIENT_RESOURCES)
	NVMLErrUnknown = NVMLReturn(C.NVML_ERROR_UNKNOWN)
)

// NVMLPCIeUtilCounter is the wrapper of nvmlPcieUtilCounter_t
// https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceStructs.html#group__nvmlDeviceStructs_1g0840d5d019555b267b45609c5568ced5
type NVMLPCIeUtilCounter int

const (
	PCIeUtilTXBytes NVMLPCIeUtilCounter = NVMLPCIeUtilCounter(C.NVML_PCIE_UTIL_TX_BYTES)
	PCIeUtilRXBytes                     = NVMLPCIeUtilCounter(C.NVML_PCIE_UTIL_RX_BYTES)
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
