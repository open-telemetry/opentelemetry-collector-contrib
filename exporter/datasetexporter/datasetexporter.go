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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type datasetExporter struct {
	limiter *rate.Limiter
	logger  *zap.Logger
	session string
}

var exporterInstance *datasetExporter

func newDatasetExporter(logger *zap.Logger) (*datasetExporter, error) {
	logger.Info("Creating new DataSet Exporter with config")
	if logger == nil {
		return nil, fmt.Errorf("logger has to be set")
	}

	return &datasetExporter{
		limiter: rate.NewLimiter(100*rate.Every(1*time.Minute), 100), // 100 requests / minute
		session: uuid.New().String(),
		logger:  logger,
	}, nil
}

var lock = &sync.Mutex{}

func getDatasetExporter(entity string, config *Config, logger *zap.Logger) (*datasetExporter, error) {
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
			instance, err := newDatasetExporter(logger)
			if err != nil {
				return nil, fmt.Errorf("cannot create new dataset exporter: %w", err)
			}
			exporterInstance = instance
		}
	}

	return exporterInstance, nil
}

func (e *datasetExporter) consumeLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}

func (e *datasetExporter) consumeTraces(ctx context.Context, ld ptrace.Traces) error {
	return nil
}
