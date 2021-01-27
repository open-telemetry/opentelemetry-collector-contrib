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

package hostmetadata

import (
	"context"
	"sync"
	"testing"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

type hostMetadataBenchmark struct {
	name   string
	cfgMod func(*HostMetadataConfig)
}

func (g *hostMetadataBenchmark) Run(b *testing.B) {
	cfg := NewHostMetadataConfig(g.name)
	g.cfgMod(cfg)
	ops, err := cfg.Build(testutil.NewBuildContext(b))
	require.NoError(b, err)
	op := ops[0]

	fake := testutil.NewFakeOutput(b)
	op.(*HostMetadata).OutputOperators = []operator.Operator{fake}

	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			e := entry.New()
			op.Process(context.Background(), e)
		}
		err = op.Stop()
		require.NoError(b, err)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			<-fake.Received
		}
	}()

	wg.Wait()
}

func BenchmarkHostMetadata(b *testing.B) {
	cases := []hostMetadataBenchmark{
		{
			"Default",
			func(cfg *HostMetadataConfig) {},
		},
		{
			"NoHostname",
			func(cfg *HostMetadataConfig) {
				cfg.IncludeHostname = false
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, tc.Run)
	}
}
