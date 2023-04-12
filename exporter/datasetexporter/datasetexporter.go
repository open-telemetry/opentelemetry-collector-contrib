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

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"fmt"
	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"github.com/scalyr/dataset-go/pkg/client"
	"github.com/scalyr/dataset-go/pkg/config"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type datasetExporter struct {
	client  *client.DataSetClient
	limiter *rate.Limiter
	logger  *zap.Logger
	session string
}

var exporterInstance *datasetExporter

func newDatasetExporter(config *config.DataSetConfig, logger *zap.Logger) (*datasetExporter, error) {
	logger.Info("Creating new DataSet Exporter with config")
	client, err := client.NewClient(
		config,
		&http.Client{Timeout: time.Second * 60},
		logger,
	)
	if err != nil {
		logger.Error("Cannot create DataSetClient: ", zap.Error(err))
		return nil, fmt.Errorf("cannot create newDatasetExporter: %w", err)
	}

	return &datasetExporter{
		client:  client,
		limiter: rate.NewLimiter(100*rate.Every(1*time.Minute), 100), // 100 requests / minute
		session: uuid.New().String(),
		logger:  logger,
	}, nil
}

var lock = &sync.Mutex{}

func getDatasetExporter(entity string, config Config, logger *zap.Logger) (*datasetExporter, error) {
	logger.Info(
		"Get logger for: ",
		zap.String("entity", entity),
	)
	// TODO: create exporter per config
	if exporterInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if exporterInstance == nil {

			logger.Info(
				"DataSetExport is using config: ",
				zap.String("config", config.String()),
				zap.String("entity", entity),
			)
			dataSetCfg, err := config.Convert()
			if err != nil {
				return nil, fmt.Errorf(
					"cannot convert config: %s; %w",
					config.String(), err,
				)
			}
			instance, err := newDatasetExporter(dataSetCfg, logger)

			if err != nil {
				return nil, fmt.Errorf("cannot create new dataset exporter: %w", err)
			}
			exporterInstance = instance
		}
	}

	return exporterInstance, nil
}

func (e *datasetExporter) shutdown() {
	e.client.SendAllAddEventsBuffers()
}

func sendBatch(events []*add_events.EventBundle, client *client.DataSetClient) error {
	return client.AddEvents(events)
}

func buildKey(prefix string, separator string, key string, depth int) string {
	res := prefix
	if depth > 0 {
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
		target[prefix] = source
	}
}
