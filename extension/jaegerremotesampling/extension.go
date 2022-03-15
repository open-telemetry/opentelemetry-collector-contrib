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

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"
)

var _ component.Extension = (*jrsExtension)(nil)

type jrsExtension struct {
	httpServer component.Component
}

func newExtension(cfg *Config, telemetry component.TelemetrySettings) (*jrsExtension, error) {
	// TODO(jpkroehling): get a proper instance
	cfgMgr := internal.NewClientConfigManager()
	ext := &jrsExtension{}

	if cfg.HTTPServerSettings != nil {
		httpServer, err := internal.NewHTTP(telemetry, *cfg.HTTPServerSettings, cfgMgr)
		if err != nil {
			return nil, err
		}
		ext.httpServer = httpServer
	}

	return ext, nil
}

func (jrse *jrsExtension) Start(ctx context.Context, host component.Host) error {
	// then we start our own server interfaces, starting with the HTTP one
	err := jrse.httpServer.Start(ctx, host)
	if err != nil {
		return err
	}

	return nil
}

func (jrse *jrsExtension) Shutdown(ctx context.Context) error {
	err := jrse.httpServer.Shutdown(ctx)
	if err != nil {
		return err
	}

	return nil
}
