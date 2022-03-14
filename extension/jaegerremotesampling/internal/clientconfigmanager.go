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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"context"

	"github.com/jaegertracing/jaeger/cmd/agent/app/configmanager"
	"github.com/jaegertracing/jaeger/thrift-gen/baggage"
	"github.com/jaegertracing/jaeger/thrift-gen/sampling"
)

// NewClientConfigManager returns a new Jaeger's configmanager.ClientConfigManager. It might be either
// a proxy to a remote location, or might serve data based on local files.
func NewClientConfigManager() configmanager.ClientConfigManager {
	return &clientCfgMgr{}
}

type clientCfgMgr struct {
}

func (m *clientCfgMgr) GetSamplingStrategy(ctx context.Context, serviceName string) (*sampling.SamplingStrategyResponse, error) {
	return sampling.NewSamplingStrategyResponse(), nil
}

func (m *clientCfgMgr) GetBaggageRestrictions(ctx context.Context, serviceName string) ([]*baggage.BaggageRestriction, error) {
	return nil, nil
}
