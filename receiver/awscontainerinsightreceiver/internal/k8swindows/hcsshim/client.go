// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package hcsshim // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/hcsshim"

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
	"go.uber.org/zap"
)

type HCSClient interface {
	GetContainerStats(containerID string) (hcsshim.Statistics, error)
	GetEndpointList() ([]hcsshim.HNSEndpoint, error)
	GetEndpointStat(endpointID string) (hcsshim.HNSEndpointStats, error)
}
type hCSClient struct {
	logger *zap.Logger
}

func (hc *hCSClient) GetContainerStats(containerID string) (hcsshim.Statistics, error) {
	container, err := hcsshim.OpenContainer(containerID)
	if err != nil {
		hc.logger.Error("failed to open container using HCS shim APIs, ", zap.Error(err))
		return hcsshim.Statistics{}, err
	}
	defer container.Close()
	cps, err := container.Statistics()
	if err != nil {
		hc.logger.Error("failed to get container stats from HCS shim APIs, ", zap.Error(err))
		return hcsshim.Statistics{}, err
	}

	return cps, nil
}

func (hc *hCSClient) GetEndpointList() ([]hcsshim.HNSEndpoint, error) {
	endpointList, err := hcsshim.HNSListEndpointRequest()
	if err != nil {
		hc.logger.Error("failed to list endpoints using HNS APIs, ", zap.Error(err))
		return []hcsshim.HNSEndpoint{}, err
	}
	return endpointList, nil
}

func (hc *hCSClient) GetEndpointStat(endpointID string) (hcsshim.HNSEndpointStats, error) {
	endpointStat, err := hcsshim.GetHNSEndpointStats(endpointID)
	if err != nil {
		hc.logger.Error("failed to get HNS endpoint stats, ", zap.Error(err))
		return hcsshim.HNSEndpointStats{}, err
	}
	if endpointStat != nil {
		return *endpointStat, nil
	}
	return hcsshim.HNSEndpointStats{}, fmt.Errorf("no stats for endpoint")
}
