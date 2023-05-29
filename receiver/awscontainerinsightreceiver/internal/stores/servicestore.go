// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

const (
	refreshIntervalService = 10 * time.Second
)

type endpointInfo interface {
	PodKeyToServiceNames() map[string][]string
}

type ServiceStore struct {
	podKeyToServiceNamesMap map[string][]string
	endpointInfo            endpointInfo
	lastRefreshed           time.Time
	logger                  *zap.Logger
}

func NewServiceStore(logger *zap.Logger) (*ServiceStore, error) {
	s := &ServiceStore{
		podKeyToServiceNamesMap: make(map[string][]string),
		logger:                  logger,
	}
	k8sClient := k8sclient.Get(logger)
	if k8sClient == nil {
		return nil, errors.New("failed to start service store because k8sclient is nil")
	}
	s.endpointInfo = k8sClient.GetEpClient()
	return s, nil
}

func (s *ServiceStore) RefreshTick(ctx context.Context) {
	now := time.Now()
	if now.Sub(s.lastRefreshed) >= refreshIntervalService {
		s.refresh(ctx)
		s.lastRefreshed = now
	}
}

// Decorate decorates metrics and update kubernetesBlob
// service info is not mandatory
func (s *ServiceStore) Decorate(_ context.Context, metric CIMetric, _ map[string]interface{}) bool {
	if metric.HasTag(ci.K8sPodNameKey) {
		podKey := createPodKeyFromMetric(metric)
		if podKey == "" {
			s.logger.Error("podKey is unavailable when decorating service.", zap.Any("podKey", podKey))
			return false
		}
		if serviceList, ok := s.podKeyToServiceNamesMap[podKey]; ok {
			if len(serviceList) > 0 {
				addServiceNameTag(metric, serviceList)
			}
		}
	}

	return true
}

func (s *ServiceStore) refresh(ctx context.Context) {
	doRefresh := func() {
		s.podKeyToServiceNamesMap = s.endpointInfo.PodKeyToServiceNames()
		s.logger.Debug("pod to service name map", zap.Any("podKeyToServiceNamesMap", s.podKeyToServiceNamesMap))
	}

	refreshWithTimeout(ctx, doRefresh, refreshIntervalService)
}

func addServiceNameTag(metric CIMetric, serviceNames []string) {
	// TODO handle serviceNames len is larger than 1. We need to duplicate the metric object
	metric.AddTag(ci.TypeService, serviceNames[0])
}
