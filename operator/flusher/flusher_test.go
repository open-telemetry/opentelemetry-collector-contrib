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

package flusher

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestFlusher(t *testing.T) {
	// Override setting for test
	maxElapsedTime = 5 * time.Second

	outChan := make(chan struct{}, 100)
	flusherCfg := NewConfig()
	flusher := flusherCfg.Build(zaptest.NewLogger(t).Sugar())

	failed := errors.New("test failure")
	for i := 0; i < 100; i++ {
		flusher.Do(func(_ context.Context) error {
			// Fail randomly but still expect the entries to come through
			if rand.Int()%5 == 0 {
				return failed
			}
			outChan <- struct{}{}
			return nil
		})
	}

	for i := 0; i < 100; i++ {
		select {
		case <-time.After(5 * time.Second):
			require.FailNow(t, "timed out")
		case <-outChan:
		}
	}
}

func TestMaxElapsedTime(t *testing.T) {
	// Override setting for test
	maxElapsedTime = 100 * time.Millisecond

	flusherCfg := NewConfig()
	flusher := flusherCfg.Build(zaptest.NewLogger(t).Sugar())

	start := time.Now()
	flusher.flushWithRetry(context.Background(), func(_ context.Context) error {
		return errors.New("never flushes")
	})
	require.WithinDuration(t, start.Add(maxElapsedTime), time.Now(), maxElapsedTime)
}
