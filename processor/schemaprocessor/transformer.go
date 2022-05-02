// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type transformer struct {
	targets []string
	log     *zap.Logger
}

func newTransformer(
	_ context.Context,
	conf config.Processor,
	set component.ProcessorCreateSettings,
) (*transformer, error) {
	cfg, ok := conf.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration provided")
	}
	return &transformer{
		log:     set.Logger,
		targets: cfg.Targets,
	}, nil
}

func (t transformer) processLogs(ctx context.Context, ld pdata.Logs) (pdata.Logs, error) {
	return ld, nil
}

func (t transformer) processMetrics(ctx context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	return md, nil
}

func (t transformer) processTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
	return td, nil
}

// start will load the remote file definition if it isn't already cached
// and resolve the schema translation file
func (t *transformer) start(ctx context.Context, host component.Host) error {
	for _, target := range t.targets {
		t.log.Info("Fetching remote schema url", zap.String("schema-url", target))
	}
	return nil
}
