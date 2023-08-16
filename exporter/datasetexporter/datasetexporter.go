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
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type DatasetExporter struct {
	client      *client.DataSetClient
	limiter     *rate.Limiter
	logger      *zap.Logger
	session     string
	exporterCfg *ExporterConfig
	serverHost  string
}

func newDatasetExporter(entity string, config *Config, set exporter.CreateSettings) (*DatasetExporter, error) {
	logger := set.Logger
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
	userAgent := fmt.Sprintf(
		"%s;%s;%s",
		"OtelCollector",
		set.BuildInfo.Version,
		entity,
	)
	client, err := client.NewClient(
		exporterCfg.datasetConfig,
		&http.Client{Timeout: time.Second * 60},
		logger,
		&userAgent,
	)
	if err != nil {
		logger.Error("Cannot create DataSetClient: ", zap.Error(err))
		return nil, fmt.Errorf("cannot create newDatasetExporter: %w", err)
	}

	return &DatasetExporter{
		client:      client,
		limiter:     rate.NewLimiter(100*rate.Every(1*time.Minute), 100), // 100 requests / minute
		session:     uuid.New().String(),
		logger:      logger,
		exporterCfg: exporterCfg,
		serverHost:  client.ServerHost(),
	}, nil
}

func (e *DatasetExporter) shutdown(context.Context) error {
	return e.client.Shutdown()
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

func inferServerHost(
	resource pcommon.Resource,
	attrs map[string]interface{},
	serverHost string,
) string {
	// first use value from the attribute serverHost
	valA, okA := attrs[add_events.AttrServerHost]
	if okA {
		host := fmt.Sprintf("%v", valA)
		if len(host) > 0 {
			return host
		}
	}

	// then use value from resource attributes - serverHost and host.name
	for _, resKey := range []string{add_events.AttrServerHost, "host.name"} {
		valR, okR := resource.Attributes().Get(resKey)
		if okR {
			host := valR.AsString()
			if len(host) > 0 {
				return host
			}
		}
	}

	return serverHost
}
