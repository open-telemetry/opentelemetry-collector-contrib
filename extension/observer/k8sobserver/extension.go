// Copyright 2020, OpenTelemetry Authors
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

package k8sobserver

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type k8sObserver struct {
	logger   *zap.Logger
	informer cache.SharedInformer
	stop     chan struct{}
	config   *Config
	server   *http.Server
}

func (k *k8sObserver) Start(ctx context.Context, host component.Host) error {
	go k.informer.Run(k.stop)

	// Use custom status port until there's a way to export status via zpages or something else.
	if k.config.StatusAddr != "" {
		k.server = &http.Server{
			Addr:    k.config.StatusAddr,
			Handler: observer.StatusMux(k),
		}
		listen, err := net.Listen("tcp", k.config.StatusAddr)
		if err != nil {
			return fmt.Errorf("unable to start status listener: %v", err)
		}
		go k.server.Serve(listen)
	}
	return nil
}

func (k *k8sObserver) Shutdown(ctx context.Context) error {
	close(k.stop)
	if k.server != nil {
		if err := k.server.Shutdown(ctx); err != nil {
			k.logger.Error("failed shutting down status server", zap.Error(err))
		}
	}
	return nil
}

var _ (component.ServiceExtension) = (*k8sObserver)(nil)

// ListAndWatch notifies watcher with the current state and sends subsequent state changes.
func (k *k8sObserver) ListAndWatch(listener observer.Notify) {
	k.informer.AddEventHandler(&handler{logger: k.logger, watcher: listener, idNamespace: k.config.Name()})
}

// newObserver creates a new k8s observer extension.
func newObserver(logger *zap.Logger, config *Config, listWatch cache.ListerWatcher) (component.ServiceExtension, error) {
	informer := cache.NewSharedInformer(listWatch, &v1.Pod{}, 0)
	return &k8sObserver{logger: logger, informer: informer, stop: make(chan struct{}), config: config}, nil
}
