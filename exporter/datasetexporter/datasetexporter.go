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
	"github.com/scalyr/dataset-go/pkg/meter_config"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type DatasetExporter struct {
	client      *client.DataSetClient
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
		zap.String("id.string", set.ID.String()),
		zap.String("id.name", set.ID.Name()),
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

	meter := set.MeterProvider.Meter("datasetexporter")
	meterConfig := meter_config.NewMeterConfig(
		&meter,
		entity,
		set.ID.Name(),
	)

	client, err := client.NewClient(
		exporterCfg.datasetConfig,
		&http.Client{Timeout: time.Second * 60},
		logger,
		&userAgent,
		meterConfig,
	)
	if err != nil {
		logger.Error("Cannot create DataSetClient: ", zap.Error(err))
		return nil, fmt.Errorf("cannot create newDatasetExporter: %w", err)
	}

	return &DatasetExporter{
		client:      client,
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

func updateWithPrefixedValuesMap(target map[string]any, prefix string, separator string, suffix string, source map[string]any, depth int) {
	for k, v := range source {
		key := buildKey(prefix, separator, k, depth)
		updateWithPrefixedValues(target, key, separator, suffix, v, depth+1)
	}
}

func updateWithPrefixedValuesArray(target map[string]any, prefix string, separator string, suffix string, source []any, depth int) {
	for i, v := range source {
		key := buildKey(prefix, separator, strconv.FormatInt(int64(i), 10), depth)
		updateWithPrefixedValues(target, key, separator, suffix, v, depth+1)
	}
}

func updateWithPrefixedValues(target map[string]any, prefix string, separator string, suffix string, source any, depth int) {
	setValue := func() {
		for {
			// now the last value wins
			// Should the first value win?
			_, found := target[prefix]
			if found && len(suffix) > 0 {
				prefix += suffix
			} else {
				target[prefix] = source
				break
			}
		}
	}

	st := reflect.TypeOf(source)
	if st == nil {
		setValue()
		return
	}
	switch st.Kind() {
	case reflect.Map:
		updateWithPrefixedValuesMap(target, prefix, separator, suffix, source.(map[string]any), depth)
	case reflect.Array, reflect.Slice:
		updateWithPrefixedValuesArray(target, prefix, separator, suffix, source.([]any), depth)
	default:
		setValue()
	}
}

func inferServerHost(
	resource pcommon.Resource,
	attrs map[string]any,
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
