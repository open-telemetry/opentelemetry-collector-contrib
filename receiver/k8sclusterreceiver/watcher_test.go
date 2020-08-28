// Copyright 2020 OpenTelemetry Authors
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

package k8sclusterreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestSetupMetadataExporters(t *testing.T) {
	type fields struct {
		metadataConsumers []metadataConsumer
	}
	type args struct {
		exporters                   map[configmodels.Exporter]component.Exporter
		metadataExportersFromConfig []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Unsupported exporter",
			fields{},
			args{
				exporters: map[configmodels.Exporter]component.Exporter{
					mockExporterConfig{ExporterName: "exampleexporter"}: MockExporter{},
				},
				metadataExportersFromConfig: []string{"exampleexporter"},
			},
			true,
		},
		{
			"Supported exporter",
			fields{
				metadataConsumers: []metadataConsumer{(&mockExporterWithK8sMetadata{}).ConsumeKubernetesMetadata},
			},
			args{exporters: map[configmodels.Exporter]component.Exporter{
				mockExporterConfig{ExporterName: "exampleexporter"}: mockExporterWithK8sMetadata{},
			},
				metadataExportersFromConfig: []string{"exampleexporter"},
			},
			false,
		},
		{
			"Non-existent exporter",
			fields{
				metadataConsumers: []metadataConsumer{},
			},
			args{exporters: map[configmodels.Exporter]component.Exporter{
				mockExporterConfig{ExporterName: "exampleexporter"}: mockExporterWithK8sMetadata{},
			},
				metadataExportersFromConfig: []string{"exampleexporter/1"},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := &resourceWatcher{
				logger: zap.NewNop(),
			}
			if err := rw.setupMetadataExporters(tt.args.exporters, tt.args.metadataExportersFromConfig); (err != nil) != tt.wantErr {
				t.Errorf("setupMetadataExporters() error = %v, wantErr %v", err, tt.wantErr)
			}

			require.Equal(t, len(tt.fields.metadataConsumers), len(rw.metadataConsumers))
		})
	}
}

func TestWaitForInitialInformerSync(t *testing.T) {
	type fields struct {
		initialCacheSyncTimeout time.Duration
		informersHaveSynced     *atomic.Bool
	}
	tests := []struct {
		name           string
		fields         fields
		wantWarning    bool
		warningMessage string
	}{
		{
			name: "Times out on initial sync with warning",
			fields: fields{
				initialCacheSyncTimeout: 1 * time.Second,
				informersHaveSynced:     atomic.NewBool(false),
			},
			wantWarning:    true,
			warningMessage: "Initial sync of informers cache not yet completed. Try increasing 'initial_cache_sync_timeout'.",
		},
		{
			name: "Returns without warning",
			fields: fields{
				initialCacheSyncTimeout: 1 * time.Second,
				informersHaveSynced:     atomic.NewBool(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedLogger, logs := observer.New(zapcore.WarnLevel)
			logger := zap.New(observedLogger)
			rw := &resourceWatcher{
				logger:                  logger,
				initialCacheSyncTimeout: tt.fields.initialCacheSyncTimeout,
				informersHaveSynced:     tt.fields.informersHaveSynced,
			}

			rw.waitForInitialInformerSync()

			if tt.wantWarning {
				require.Len(t, logs.All(), 1)
				assert.Equal(t, tt.warningMessage, logs.All()[0].Message)
				return
			}

			require.Len(t, logs.All(), 0)
		})
	}
}
