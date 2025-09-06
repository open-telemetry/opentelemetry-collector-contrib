// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kedascalerexporter

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

const (
	MetricStoreValueIfNotFound = 0
	MetadataQuery              = "query"
	MetadataTargetValue        = "targetValue"
)

type kedaScaler struct {
	metricStore    *OTLPMetricStore
	logger         *zap.Logger
	streamInterval time.Duration
	externalscaler.UnimplementedExternalScalerServer
}

func newKedaScaler(
	logger *zap.Logger,
	metricStore *OTLPMetricStore,
) externalscaler.ExternalScalerServer {
	return &kedaScaler{
		logger:         logger,
		streamInterval: 5 * time.Second,
		metricStore:    metricStore,
	}
}

func (e *kedaScaler) IsActive(
	_ context.Context,
	sor *externalscaler.ScaledObjectRef,
) (*externalscaler.IsActiveResponse, error) {
	value, err := e.getMetric(sor)
	if err != nil {
		e.logger.Error(
			"getMetric failed",
			zap.Error(err),
			zap.String("scaledObjectRef", sor.String()),
		)
		return nil, err
	}

	active := value > 0
	return &externalscaler.IsActiveResponse{Result: active}, nil
}

func (e *kedaScaler) StreamIsActive(
	scaledObject *externalscaler.ScaledObjectRef,
	server externalscaler.ExternalScaler_StreamIsActiveServer,
) error {
	// this function communicates with KEDA via the 'server' parameter.
	// we call server.Send (below) every streamInterval, which tells it to immediately
	// ping our IsActive RPC
	ticker := time.NewTicker(e.streamInterval)
	defer ticker.Stop()
	for {
		select {
		case <-server.Context().Done():
			return nil
		case <-ticker.C:
			active, err := e.IsActive(server.Context(), scaledObject)
			if err != nil {
				e.logger.Error("error getting active status in stream", zap.Error(err))
				return err
			}
			err = server.Send(&externalscaler.IsActiveResponse{
				Result: active.Result,
			})
			if err != nil {
				e.logger.Error("error sending the active result in stream", zap.Error(err))
				return err
			}
		}
	}
}

func (e *kedaScaler) GetMetrics(
	_ context.Context,
	metricRequest *externalscaler.GetMetricsRequest,
) (*externalscaler.GetMetricsResponse, error) {
	sor := metricRequest.ScaledObjectRef
	value, err := e.getMetric(sor)
	if err != nil {
		return nil, err
	}

	res := &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:       metricRequest.GetMetricName(),
				MetricValue:      int64(math.Ceil(value)),
				MetricValueFloat: math.Ceil(value),
			},
		},
	}

	return res, nil
}

func (e *kedaScaler) GetMetricSpec(
	_ context.Context,
	sor *externalscaler.ScaledObjectRef,
) (*externalscaler.GetMetricSpecResponse, error) {
	namespacedName := namespacedNameFromScaledObjectRef(sor)
	kedaMetricName := fmt.Sprintf("%s-%s", namespacedName.Namespace, namespacedName.Name)

	scalerMetadata := sor.GetScalerMetadata()
	if scalerMetadata == nil {
		e.logger.Info(
			"unable to get SO metadata",
			zap.String("name", sor.Name),
			zap.String("namespace", sor.Namespace),
		)
		return nil, fmt.Errorf("GetMetricSpec")
	}
	targetValue, err := getTargetValue(scalerMetadata)
	if err != nil {
		e.logger.Error(
			"unable to get target value from SO metadata",
			zap.Error(err),
			zap.String("name", sor.Name),
			zap.String("namespace", sor.Namespace),
		)
		return nil, err
	}

	res := &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{
				MetricName:      kedaMetricName,
				TargetSize:      targetValue,
				TargetSizeFloat: float64(targetValue),
			},
		},
	}
	e.logger.Info(
		"GetMetricSpec",
		zap.String("metricName", kedaMetricName),
		zap.Int64("targetValue", targetValue),
	)
	return res, nil
}

func namespacedNameFromScaledObjectRef(sor *externalscaler.ScaledObjectRef) *types.NamespacedName {
	if sor == nil {
		return nil
	}

	return &types.NamespacedName{
		Namespace: sor.GetNamespace(),
		Name:      sor.GetName(),
	}
}

func getTargetValue(metadata map[string]string) (int64, error) {
	targetValueStr, found := metadata[MetadataTargetValue]
	if !found {
		return -1, fmt.Errorf("not found %s", MetadataTargetValue)
	}
	targetValue, err := strconv.ParseInt(targetValueStr, 10, 64)
	if err != nil {
		return -1, err
	}
	return targetValue, nil
}

func (e *kedaScaler) getMetricQuery(metadata map[string]string) (string, error) {
	metricQuery, found := metadata[MetadataQuery]
	if !found {
		e.logger.Error(
			"unable to get metric query from scaled object's metadata",
			zap.String("metadata", fmt.Sprintf("%v", metadata)),
		)
		return "", fmt.Errorf("unable to get metric query from scaled object's metadata")
	}

	metricQuery = strings.TrimSpace(metricQuery)
	return metricQuery, nil
}

func (e *kedaScaler) getMetric(sor *externalscaler.ScaledObjectRef) (float64, error) {
	query, err := e.getMetricQuery(sor.GetScalerMetadata())
	if err != nil {
		return MetricStoreValueIfNotFound, err
	}

	val, err := e.metricStore.Query(query, time.Now())
	if err != nil {
		return MetricStoreValueIfNotFound, err
	}

	return val, nil
}
