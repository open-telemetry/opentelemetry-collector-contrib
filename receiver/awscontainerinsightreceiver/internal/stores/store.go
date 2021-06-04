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

package stores

import (
	"os"
	"time"

	"go.uber.org/zap"
)

// CIMetric represents the raw metric interface for container insights
type CIMetric interface {
	HasField(key string) bool
	AddField(key string, val interface{})
	GetField(key string) interface{}
	HasTag(key string) bool
	AddTag(key, val string)
	GetTag(key string) string
	RemoveTag(key string)
}

type K8sStore interface {
	Decorate(metric CIMetric, kubernetesBlob map[string]interface{}) bool
	RefreshTick()
}

var stores []K8sStore

// TODO: add code to initialize pod and service store and provide api for decorating metrics
func DecorateMetrics() {
	// NodeName := os.Getenv("HOST_NAME")
	// if NodeName == "" {
	// 	return errors.New("missing environment variable HOST_NAME. Please check your YAML config")
	// }

	HostIP := os.Getenv("HOST_IP")
	// if HostIP == "" {
	// 	return errors.New("missing environment variable HOST_IP. Please check your YAML config")
	// }

	shutdownC := make(chan bool)

	PrefFullPodName := false
	logger := zap.NewNop()
	podstore, _ := NewPodStore(HostIP, PrefFullPodName, logger)
	stores = append(stores, podstore)
	TagService := true
	if TagService {
		stores = append(stores, NewServiceStore(logger))
	}

	for _, store := range stores {
		store.RefreshTick()
	}

	go func() {
		refreshTicker := time.NewTicker(time.Second)
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				for _, store := range stores {
					store.RefreshTick()
				}
			case <-shutdownC:
				refreshTicker.Stop()
				return
			}
		}
	}()
}
