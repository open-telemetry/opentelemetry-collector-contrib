// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/gpu"

const (
	gpuUtil           = "DCGM_FI_DEV_GPU_UTIL"
	gpuMemUtil        = "DCGM_FI_DEV_FB_USED_PERCENT"
	gpuMemUsed        = "DCGM_FI_DEV_FB_USED"
	gpuMemTotal       = "DCGM_FI_DEV_FB_TOTAL"
	gpuTemperature    = "DCGM_FI_DEV_GPU_TEMP"
	gpuPowerDraw      = "DCGM_FI_DEV_POWER_USAGE"
	gpuTensorCoreUtil = "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE"
)

var MetricToUnit = map[string]string{
	gpuUtil:           "Percent",
	gpuMemUtil:        "Percent",
	gpuMemUsed:        "Bytes",
	gpuMemTotal:       "Bytes",
	gpuTemperature:    "None",
	gpuPowerDraw:      "None",
	gpuTensorCoreUtil: "Percent",
}
