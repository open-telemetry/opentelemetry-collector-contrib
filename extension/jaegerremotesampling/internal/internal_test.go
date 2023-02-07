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

package internal

import (
	"context"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

type mockCfgMgr struct {
	getSamplingStrategyFunc func(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error)
}

func (m *mockCfgMgr) GetSamplingStrategy(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error) {
	if m.getSamplingStrategyFunc != nil {
		return m.getSamplingStrategyFunc(ctx, serviceName)
	}
	return &api_v2.SamplingStrategyResponse{StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC}, nil
}
