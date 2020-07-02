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

// NVMLInit is the wrapper of nvmlInit()
func NVMLInit() {
	C.nvmlInit()
}

// NVMLDeviceGetHandledByIndex is the wrapper of nvmlDeviceGetHandleByIndex()
func NVMLDeviceGetHandledByIndex(gpuID uint64) C.nvmlDevice_t {
	var device C.nvmlDevice_t
	C.nvmlDeviceGetHandleByIndex(C.uint(gpuID), &device)
	return device
}

// NVMLDeviceGetTemperature is the wrapper of nvmlDeviceGetTemperature
func NVMLDeviceGetTemperature(device C.nvmlDevice_t) uint64 {
	var temp C.uint
	C.nvmlDeviceGetTemperature(device, C.NVML_TEMPERATURE_GPU, &temp)
	return uint64(temp)
}

// NVMLDeviceGetPowerUsage is the wrapper of nvmlDeviceGetPowerUsage
func NVMLDeviceGetPowerUsage(device C.nvmlDevice_t) uint64 {
	var power C.uint
	C.nvmlDeviceGetPowerUsage(device, &power)
	return uint64(power)
}
