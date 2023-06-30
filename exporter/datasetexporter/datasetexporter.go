// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"github.com/scalyr/dataset-go/pkg/client"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type DatasetExporter struct {
	client      *client.DataSetClient
	limiter     *rate.Limiter
	logger      *zap.Logger
	session     string
	spanTracker *spanTracker
	exporterCfg *ExporterConfig
}

func newDatasetExporter(entity string, config *Config, logger *zap.Logger) (*DatasetExporter, error) {
	logger.Info("Creating new DataSetExporter",
		zap.String("config", config.String()),
		zap.String("entity", entity),
	)
	exporterCfg, err := config.convert()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert config: %s; %w",
			config.String(), err,
		)
	}

	client, err := client.NewClient(
		exporterCfg.datasetConfig,
		&http.Client{Timeout: time.Second * 60},
		logger,
	)
	if err != nil {
		logger.Error("Cannot create DataSetClient: ", zap.Error(err))
		return nil, fmt.Errorf("cannot create newDatasetExporter: %w", err)
	}

	tracker := newSpanTracker(exporterCfg.tracesSettings.MaxWait)
	if !exporterCfg.tracesSettings.Aggregate {
		tracker = nil
	}

	return &DatasetExporter{
		client:      client,
		limiter:     rate.NewLimiter(100*rate.Every(1*time.Minute), 100), // 100 requests / minute
		session:     uuid.New().String(),
		logger:      logger,
		spanTracker: tracker,
		exporterCfg: exporterCfg,
	}, nil
}

func (e *DatasetExporter) shutdown(context.Context) error {
	e.client.SendAllAddEventsBuffers()
	return nil
}

func sendBatch(events []*add_events.EventBundle, client *client.DataSetClient) error {
	return client.AddEvents(events)
}

func buildKey(prefix string, separator string, key string, depth int) string {
	res := prefix
	if depth > 0 && len(prefix) > 0 {
		res += separator
	}
	res += key
	return res
}

func updateWithPrefixedValuesMap(target map[string]interface{}, prefix string, separator string, source map[string]interface{}, depth int) {
	for k, v := range source {
		key := buildKey(prefix, separator, k, depth)
		updateWithPrefixedValues(target, key, separator, v, depth+1)
	}
}

func updateWithPrefixedValuesArray(target map[string]interface{}, prefix string, separator string, source []interface{}, depth int) {
	for i, v := range source {
		key := buildKey(prefix, separator, strconv.FormatInt(int64(i), 10), depth)
		updateWithPrefixedValues(target, key, separator, v, depth+1)
	}
}

func updateWithPrefixedValues(target map[string]interface{}, prefix string, separator string, source interface{}, depth int) {
	st := reflect.TypeOf(source)
	switch st.Kind() {
	case reflect.Map:
		updateWithPrefixedValuesMap(target, prefix, separator, source.(map[string]interface{}), depth)
	case reflect.Array, reflect.Slice:
		updateWithPrefixedValuesArray(target, prefix, separator, source.([]interface{}), depth)
	default:
		for {
			_, found := target[prefix]
			if found {
				prefix += separator
			} else {
				target[prefix] = source
				break
			}
		}

	}
}
