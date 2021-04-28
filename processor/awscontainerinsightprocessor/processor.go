// Copyright  OpenTelemetry Authors
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

package awscontainerinsightprocessor

import (
	"context"
	"errors"
	"os"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awscontainerinsightprocessor/internal/stores"
)

type awscontainerinsightprocessor struct {
	logger                  *zap.Logger
	stores                  []stores.K8sStore
	storeRefreshMinInterval time.Duration
	storeRefreshTimeout     time.Duration
	storeRefreshStoppedC    chan bool // used only by test
	shutdownC               chan bool
	TagService              bool   `toml:"tag_service"`
	ClusterName             string `toml:"cluster_name"`
	HostIP                  string `toml:"host_ip"`
	NodeName                string `toml:"node_name"`
	PrefFullPodName         bool   `toml:"prefer_full_pod_name"`
}

// ProcessMetrics takes pdata.Metrics and does metric decorations (e.g. adding resource attributes/new metrics)
func (acip *awscontainerinsightprocessor) ProcessMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	//TODO: add logic to do the metric decoration by adding new resource attributes and datapoints
	return md, nil
}

// Shutdown stops the aws container insights processor
func (acip *awscontainerinsightprocessor) Shutdown(_ context.Context) error {
	close(acip.shutdownC)
	return nil
}

func (acip *awscontainerinsightprocessor) refreshStoreWithTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), acip.storeRefreshTimeout)
	// spawn a goroutine to process the stores
	go func(ctx context.Context, cancel func()) {
		for _, store := range acip.stores {
			store.RefreshTick(ctx)
		}
		cancel()
	}(ctx, cancel)
	// block until either the goroutine has processed all stores or the timeout expires
	<-ctx.Done()
	cancel()
}

func (acip *awscontainerinsightprocessor) init() error {
	acip.NodeName = os.Getenv("HOST_NAME")
	if acip.NodeName == "" {
		return errors.New("missing environment variable HOST_NAME. Please check your deployment YAML file")
	}

	acip.HostIP = os.Getenv("HOST_IP")
	if acip.HostIP == "" {
		return errors.New("missing environment variable HOST_IP. Please check your deployment YAML file")
	}

	acip.shutdownC = make(chan bool)

	// TODO: add the relevant store to acip.stores
	acip.refreshStoreWithTimeout()

	go func() {
		refreshTicker := time.NewTicker(acip.storeRefreshMinInterval)
		for {
			select {
			case <-refreshTicker.C:
				acip.refreshStoreWithTimeout()
			case <-acip.shutdownC:
				//for testing only
				if acip.storeRefreshStoppedC != nil {
					close(acip.storeRefreshStoppedC)
				}
				refreshTicker.Stop()
				return
			}
		}
	}()

	return nil
}

func newAwsContainerInsightProcessor(logger *zap.Logger, config *Config) *awscontainerinsightprocessor {
	processor := &awscontainerinsightprocessor{
		logger:                  logger,
		TagService:              config.TagService,
		PrefFullPodName:         config.PrefFullPodName,
		storeRefreshMinInterval: time.Second,
		storeRefreshTimeout:     30 * time.Second, //make it big enough so that all stores can finish refreshing
	}
	if err := processor.init(); err != nil {
		processor.logger.Error("Fail to start awscontainerinsightprocessor", zap.Error(err))
		return nil
	}
	return processor
}
