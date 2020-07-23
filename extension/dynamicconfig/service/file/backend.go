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

package file

import (
	"fmt"
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.opentelemetry.io/collector/config"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/model"
	pb "github.com/open-telemetry/opentelemetry-proto/gen/go/experimental/metricconfigservice"
	res "github.com/open-telemetry/opentelemetry-proto/gen/go/resource/v1"
)

// A Backend is a ConfigBackend that uses a local file to determine what
// schedules to change. The file is read live, so changes to it will reflect
// immediately in the configs.
type Backend struct {
	viper *viper.Viper

	mu          sync.Mutex
	configModel *model.Config // protected by mutex

	waitTime int32
	updateCh chan struct{} // syncs updates; meant for testing
}

func NewBackend(configFile string) (*Backend, error) {
	backend := &Backend{
		viper:    config.NewViper(),
		waitTime: 30,
	}
	backend.viper.SetConfigFile(configFile)

	if err := backend.viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("local backend failed to read config: %w", err)
	}

	if err := backend.updateConfig(); err != nil {
		return nil, err
	}

	backend.viper.WatchConfig()
	backend.viper.OnConfigChange(func(e fsnotify.Event) {
		if err := backend.updateConfig(); err != nil {
			log.Printf("failed to update configs: %v", err)
		}

		if backend.updateCh != nil {
			backend.updateCh <- struct{}{}
		}
	})

	return backend, nil
}

func (backend *Backend) updateConfig() error {
	var configModel model.Config
	if err := backend.viper.UnmarshalExact(&configModel); err != nil {
		return fmt.Errorf("file backend failed to decode config: %w", err)
	}

	backend.mu.Lock()
	defer backend.mu.Unlock()

	backend.configModel = &configModel
	return nil
}

// BuildConfigResponse builds a MetricConfigResponse based on the config data
// and backend settings.
func (backend *Backend) BuildConfigResponse(resource *res.Resource) (*pb.MetricConfigResponse, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()

	configBlock := backend.configModel.Match(resource)
	schedulesProto, err := configBlock.Proto()
	if err != nil {
		return nil, err
	}

	resp := &pb.MetricConfigResponse{
		Fingerprint:          configBlock.Hash(),
		Schedules:            schedulesProto,
		SuggestedWaitTimeSec: backend.waitTime,
	}

	return resp, nil
}

func (backend *Backend) GetWaitTime() int32 {
	return backend.waitTime
}

func (backend *Backend) SetWaitTime(waitTime int32) {
	if waitTime > 0 {
		backend.waitTime = waitTime
	}
}

func (backend *Backend) Close() error {
	// TODO: need to cleanup Viper resources?
	return nil
}
