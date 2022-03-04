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

package schemaprocessor

import (
	"context"
	"errors"
	"net/url"
	"path"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	schemas "go.opentelemetry.io/otel/schema/v1.0"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), config.Type(typeStr))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
	})
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func readTestSchema(_ context.Context, schemaURL string) (*ast.Schema, error) {
	u, err := url.Parse(schemaURL)
	if err != nil {
		panic(err)
	}
	return schemas.ParseFile(path.Join("testdata", u.Path))
}

func TestFactory_GetSchemaConcurrently(t *testing.T) {
	factory := newFactory(readTestSchema)

	wg := sync.WaitGroup{}
	var firstSchema *ast.Schema
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			schema, err := factory.getSchema(
				context.Background(), "https://opentelemetry.io/schemas/1.7.0",
			)
			require.NoError(t, err)
			require.NotNil(t, schema)
			if firstSchema == nil {
				firstSchema = schema
			} else {
				// All getSchema calls must return the same instance even when called
				// concurrently.
				assert.Equal(t, firstSchema, schema)
			}
		}()
	}
	wg.Wait()
}

func TestFactory_GetSchemaFail(t *testing.T) {
	factory := newFactory(readTestSchema)
	schema, err := factory.getSchema(
		context.Background(), "https://opentelemetry.io/schemas/11.22.33",
	)
	assert.Error(t, err)
	assert.Nil(t, schema)
}

func readBlockSchema(context context.Context, _ string) (*ast.Schema, error) {
	select {
	case <-context.Done():
		return nil, errors.New("schema fetching cancelled")
	}
}

func TestFactory_GetSchemaCancel(t *testing.T) {
	factory := newFactory(readBlockSchema)

	ctx, cancelFunc := context.WithCancel(context.Background())

	var processing uint64
	var cancelling uint64
	go func() {
		assert.Equal(t, uint64(0), atomic.LoadUint64(&cancelling))

		atomic.AddUint64(&processing, 1)
		schema, err := factory.getSchema(
			ctx, "https://opentelemetry.io/schemas/1.7.0",
		)
		assert.Equal(t, uint64(1), atomic.LoadUint64(&cancelling))

		atomic.AddUint64(&processing, 1)
		assert.Error(t, err)
		assert.Nil(t, schema)
	}()

	assert.Eventually(t, func() bool { return atomic.LoadUint64(&processing) == 1 }, time.Second, time.Millisecond)

	atomic.AddUint64(&cancelling, 1)
	cancelFunc()

	assert.Eventually(t, func() bool { return atomic.LoadUint64(&processing) == 2 }, time.Second, time.Millisecond)
}

func TestFactory_ConsumeLogs(t *testing.T) {
	factory := newFactoryWithFetcher(readTestSchema)
	cfg := factory.CreateDefaultConfig()
	sink := &consumertest.LogsSink{}
	proc, err := factory.CreateLogsProcessor(context.Background(), component.ProcessorCreateSettings{}, cfg, sink)
	require.NoError(t, err)
	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
}
