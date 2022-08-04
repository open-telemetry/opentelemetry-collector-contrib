// Copyright The OpenTelemetry Authors
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

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/gohai"

import (
	"github.com/DataDog/gohai/cpu"
	"github.com/DataDog/gohai/filesystem"
	"github.com/DataDog/gohai/memory"
	"github.com/DataDog/gohai/network"
	"github.com/DataDog/gohai/platform"
	"go.uber.org/zap"
)

// GetPayload builds a payload of every metadata collected with gohai except processes metadata.
// Parts of this are based on datadog-agent code
// https://github.com/DataDog/datadog-agent/blob/94a28d9cee3f1c886b3866e8208be5b2a8c2c217/pkg/metadata/internal/gohai/gohai.go#L27-L32
func GetPayload(logger *zap.Logger) Payload {
	return Payload{
		Gohai: MarshalledGohaiPayload{
			gohai: getGohaiInfo(logger),
		},
	}
}

func getGohaiInfo(logger *zap.Logger) *gohai {
	res := new(gohai)

	cpuPayload, err := new(cpu.Cpu).Collect()
	if err == nil {
		res.CPU = cpuPayload
	} else {
		logger.Warn("Failed to retrieve cpu metadata", zap.Error(err))
	}

	fileSystemPayload, err := new(filesystem.FileSystem).Collect()
	if err == nil {
		res.FileSystem = fileSystemPayload
	} else {
		logger.Warn("Failed to retrieve filesystem metadata", zap.Error(err))
	}

	memoryPayload, err := new(memory.Memory).Collect()
	if err == nil {
		res.Memory = memoryPayload
	} else {
		logger.Warn("Failed to retrieve memory metadata", zap.Error(err))
	}

	// in case of containerized environment , this would return pod id not node's ip
	networkPayload, err := new(network.Network).Collect()
	if err == nil {
		res.Network = networkPayload
	} else {
		logger.Warn("Failed to retrieve network metadata", zap.Error(err))
	}

	platformPayload, err := new(platform.Platform).Collect()
	if err == nil {
		res.Platform = platformPayload
	} else {
		logger.Warn("Failed to retrieve platform metadata", zap.Error(err))
	}

	return res
}
