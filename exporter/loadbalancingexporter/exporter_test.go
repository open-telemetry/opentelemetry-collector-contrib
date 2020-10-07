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

package loadbalancingexporter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestExporterCapabilities(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)

	// test
	caps := p.GetCapabilities()

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, false, caps.MutatesConsumedData)
}

func TestStart(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)
	require.NotNil(t, p)
	require.NoError(t, err)
	p.res = &mockResolver{}

	// test
	res := p.Start(context.Background(), componenttest.NewNopHost())
	defer p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestShutdown(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestConsumeTraces(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)
	require.NotNil(t, p)
	require.NoError(t, err)
	p.res = &mockResolver{}

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	// test
	res := p.ConsumeTraces(context.Background(), pdata.NewTraces())

	// verify
	assert.Nil(t, res)
}

func TestFailedToResolveAtStartup(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)
	require.NotNil(t, p)
	require.NoError(t, err)

	expectedErr := errors.New("some error")
	p.res = &mockResolver{
		onResolve: func(_ context.Context) ([]string, error) {
			return nil, expectedErr
		},
	}

	// test
	err = p.Start(context.Background(), componenttest.NewNopHost())

	// verify
	assert.Equal(t, expectedErr, err)
}

func TestResolveAndUpdate(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)
	require.NotNil(t, p)
	require.NoError(t, err)

	counter := -1
	results := [][]string{
		{"endpoint-1"},               // results for the first call
		{"endpoint-1", "endpoint-2"}, // results for the second call
	}
	p.res = &mockResolver{
		onResolve: func(_ context.Context) ([]string, error) {
			if counter > 1 {
				return results[1], nil
			}
			counter++
			return results[counter], nil
		},
	}

	// test
	err = p.resolveAndUpdate(context.Background())
	require.NoError(t, err)
	require.Len(t, p.ring.items, defaultWeight)

	// this should resolve to two endpoints
	err = p.resolveAndUpdate(context.Background())

	// verify
	assert.NoError(t, err)
	assert.Len(t, p.ring.items, 2*defaultWeight)
}

func TestPeriodicallyResolve(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)
	require.NotNil(t, p)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(2)
	counter := -1
	results := [][]string{
		{"endpoint-1"},               // results for the first call
		{"endpoint-1", "endpoint-2"}, // results for the second call
	}
	p.res = &mockResolver{
		onResolve: func(_ context.Context) ([]string, error) {
			if counter > 1 {
				return results[1], nil
			}
			counter++
			wg.Done()
			return results[counter], nil
		},
	}
	p.resInterval = 50 * time.Millisecond

	// test
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	require.Len(t, p.ring.items, defaultWeight)

	// wait for at least one periodic routine to be called
	wg.Wait()

	// wait for the periodic routine to be completed
	err = p.Shutdown(context.Background())
	require.NoError(t, err)

	// verify
	assert.Len(t, p.ring.items, 2*defaultWeight)
}

func TestFailedToPeriodicallyResolve(t *testing.T) {
	// prepare
	config := &Config{}
	p, err := newExporter(zap.NewNop(), config)
	require.NotNil(t, p)
	require.NoError(t, err)

	expectedErr := errors.New("expected err")
	wg := sync.WaitGroup{}
	wg.Add(2)
	once := sync.Once{}
	counter := 0
	p.res = &mockResolver{
		onResolve: func(_ context.Context) ([]string, error) {
			if counter > 0 {
				once.Do(func() {
					wg.Done()
				})
				return nil, expectedErr
			}
			counter++
			wg.Done()
			return []string{"endpoint-1"}, nil
		},
	}
	p.resInterval = 50 * time.Millisecond

	// test
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	require.Len(t, p.ring.items, defaultWeight)

	// wait for at least one periodic routine to be called
	wg.Wait()

	// wait for the periodic routine to be completed
	err = p.Shutdown(context.Background())
	require.NoError(t, err)

	// verify
	assert.Len(t, p.ring.items, defaultWeight)
}
