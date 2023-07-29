// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/gohai"

import (
	"github.com/DataDog/gohai/cpu"
	"github.com/DataDog/gohai/filesystem"
	"github.com/DataDog/gohai/memory"
	"github.com/DataDog/gohai/network"
	"github.com/DataDog/gohai/platform"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata/gohai"
	"go.uber.org/zap"
)

// NewPayload builds a payload of every metadata collected with gohai except processes metadata.
// Parts of this are based on datadog-agent code
// https://github.com/DataDog/datadog-agent/blob/94a28d9cee3f1c886b3866e8208be5b2a8c2c217/pkg/metadata/internal/gohai/gohai.go#L27-L32
func NewPayload(logger *zap.Logger) gohai.Payload {
	payload := gohai.NewEmpty()
	payload.Gohai.Gohai = newGohai(logger)
	return payload
}

func newGohai(logger *zap.Logger) *gohai.Gohai {
	res := new(gohai.Gohai)

	if p, err := new(cpu.Cpu).Collect(); err != nil {
		logger.Info("Failed to retrieve cpu metadata", zap.Error(err))
	} else {
		res.CPU = p
	}

	if p, err := new(filesystem.FileSystem).Collect(); err != nil {
		logger.Info("Failed to retrieve filesystem metadata", zap.Error(err))
	} else {
		res.FileSystem = p
	}

	if p, err := new(memory.Memory).Collect(); err != nil {
		logger.Info("Failed to retrieve memory metadata", zap.Error(err))
	} else {
		res.Memory = p
	}

	// in case of containerized environment, this would return pod id not node's ip
	if p, err := new(network.Network).Collect(); err != nil {
		logger.Info("Failed to retrieve network metadata", zap.Error(err))
	} else {
		res.Network = p
	}

	if p, err := new(platform.Platform).Collect(); err != nil {
		logger.Info("Failed to retrieve platform metadata", zap.Error(err))
	} else {
		res.Platform = p
	}

	return res
}
