// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaegerremotesampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"

import (
	"context"
	"errors"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"
	"go.opentelemetry.io/collector/component"
)

var (
	errAlreadyStarted = errors.New("the extension has been started already")
	errNotYetStarted  = errors.New("the extension has not been started yet")
)

var _ component.Extension = (*jrsExtension)(nil)

type jrsExtension struct {
	started    bool
	startedMu  sync.RWMutex
	httpServer component.Component
}

func newExtension(cfg *Config, telemetry component.TelemetrySettings) (*jrsExtension, error) {
	// TODO(jpkroehling): get a proper instance
	var cfgMgr = internal.NewClientConfigManager()

	httpServer, err := internal.NewHTTP(telemetry, cfg.HTTPServerSettings, cfgMgr)
	if err != nil {
		return nil, err
	}

	return &jrsExtension{
		httpServer: httpServer,
	}, nil
}

func (jrse *jrsExtension) Start(ctx context.Context, host component.Host) error {
	jrse.startedMu.RLock()
	if jrse.started {
		return errAlreadyStarted
	}
	jrse.startedMu.RUnlock()

	jrse.startedMu.Lock()
	defer jrse.startedMu.Unlock()
	jrse.started = true

	// then we start our own server interfaces, starting with the HTTP one
	err := jrse.httpServer.Start(ctx, host)
	if err != nil {
		return err
	}

	return nil
}

func (jrse *jrsExtension) Shutdown(ctx context.Context) error {
	jrse.startedMu.RLock()
	if !jrse.started {
		return errNotYetStarted
	}
	jrse.startedMu.RUnlock()

	err := jrse.httpServer.Shutdown(ctx)
	if err != nil {
		return err
	}

	return nil
}
