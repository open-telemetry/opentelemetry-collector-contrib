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

package buffer

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"testing"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func newMemoryBuffer(t testing.TB) *MemoryBuffer {
	b, err := NewMemoryBufferConfig().Build(testutil.NewBuildContext(t), "test")
	require.NoError(t, err)
	return b.(*MemoryBuffer)
}

func TestMemoryBuffer(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		writeN(t, b, 1, 0)
		readN(t, b, 1, 0)
	})

	t.Run("Write2Read1Read1", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		writeN(t, b, 2, 0)
		readN(t, b, 1, 0)
		readN(t, b, 1, 1)
	})

	t.Run("Write20Read10Read10", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		writeN(t, b, 20, 0)
		readN(t, b, 10, 0)
		readN(t, b, 10, 10)
	})

	t.Run("CheckN", func(t *testing.T) {
		t.Run("Read", func(t *testing.T) {
			b := newMemoryBuffer(t)
			writeN(t, b, 12, 0)
			dst := make([]*entry.Entry, 30)
			_, n, err := b.Read(dst)
			require.NoError(t, err)
			require.Equal(t, n, 12)
		})

		t.Run("ReadWait", func(t *testing.T) {
			b := newMemoryBuffer(t)
			writeN(t, b, 12, 0)
			dst := make([]*entry.Entry, 30)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, n, err := b.ReadWait(ctx, dst)
			require.NoError(t, err)
			require.Equal(t, 12, n)
		})
	})

	t.Run("SingleReadWaitMultipleWrites", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		writeN(t, b, 10, 0)
		readyDone := make(chan struct{})
		go func() {
			readyDone <- struct{}{}
			readWaitN(t, b, 20)
			readyDone <- struct{}{}
		}()
		<-readyDone
		time.Sleep(100 * time.Millisecond)
		writeN(t, b, 10, 10)
		<-readyDone
	})

	t.Run("ReadWaitOnlyWaitForPartialWrite", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		writeN(t, b, 10, 0)
		readyDone := make(chan struct{})
		go func() {
			readyDone <- struct{}{}
			readWaitN(t, b, 15)
			readyDone <- struct{}{}
		}()
		<-readyDone
		writeN(t, b, 10, 10)
		<-readyDone
		readN(t, b, 5, 15)
	})

	t.Run("Write10Read10Read0", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		writeN(t, b, 10, 0)
		readN(t, b, 10, 0)
		dst := make([]*entry.Entry, 10)
		_, n, err := b.Read(dst)
		require.NoError(t, err)
		require.Equal(t, 0, n)
	})

	t.Run("Write20Read10Read10Unfull", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		writeN(t, b, 20, 0)
		readN(t, b, 10, 0)
		dst := make([]*entry.Entry, 20)
		_, n, err := b.Read(dst)
		require.NoError(t, err)
		require.Equal(t, 10, n)
	})

	t.Run("ReadWaitTimesOut", func(t *testing.T) {
		t.Parallel()
		b := newMemoryBuffer(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		dst := make([]*entry.Entry, 10)
		_, n, err := b.ReadWait(ctx, dst)
		require.NoError(t, err)
		require.Equal(t, 0, n)
	})

	t.Run("WriteCloseRead", func(t *testing.T) {
		t.Parallel()
		bc := testutil.NewBuildContext(t)
		b, err := NewMemoryBufferConfig().Build(bc, "test")
		require.NoError(t, err)
		writeN(t, b, 10, 0)
		flushN(t, b, 5, 0)

		// Close the buffer, writing to the database
		require.NoError(t, b.Close())

		// Reopen, and expect to be able to read 5 logs
		b, err = NewMemoryBufferConfig().Build(bc, "test")
		require.NoError(t, err)

		readN(t, b, 5, 5)
		dst := make([]*entry.Entry, 10)
		_, n, err := b.Read(dst)
		require.NoError(t, err)
		require.Equal(t, 0, n)
	})

	t.Run("AddTimesOut", func(t *testing.T) {
		t.Parallel()
		cfg := MemoryBufferConfig{
			MaxEntries: 1,
		}
		b, err := cfg.Build(testutil.NewBuildContext(t), "test")
		require.NoError(t, err)

		// Add a first entry
		err = b.Add(context.Background(), entry.New())
		require.NoError(t, err)

		// Second entry should block and be cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		err = b.Add(ctx, entry.New())
		require.Error(t, err)
		cancel()

		// Read and flush
		dst := make([]*entry.Entry, 1)
		c, n, err := b.Read(dst)
		require.NoError(t, err)
		require.Equal(t, 1, n)
		require.NoError(t, c.MarkAllAsFlushed())

		// Now there should be space for another entry
		err = b.Add(context.Background(), entry.New())
		require.NoError(t, err)
	})

	t.Run("Write10kRandom", func(t *testing.T) {
		t.Parallel()
		rand.Seed(time.Now().Unix())
		seed := rand.Int63()
		t.Run(strconv.Itoa(int(seed)), func(t *testing.T) {
			t.Parallel()
			r := rand.New(rand.NewSource(seed))

			b := newMemoryBuffer(t)

			writes := 0
			reads := 0

			for i := 0; i < 10000; i++ {
				j := r.Int() % 1000
				switch {
				case j < 900:
					writeN(t, b, 1, writes)
					writes++
				default:
					readCount := (writes - reads) / 2
					c := readN(t, b, readCount, reads)
					if j%2 == 0 {
						require.NoError(t, c.MarkAllAsFlushed())
					}
					reads += readCount
				}
			}
		})
	})

	t.Run("CloseReadUnflushed", func(t *testing.T) {
		t.Parallel()
		buildContext := testutil.NewBuildContext(t)
		b, err := NewMemoryBufferConfig().Build(buildContext, "test")
		require.NoError(t, err)

		writeN(t, b, 20, 0)
		readN(t, b, 5, 0)
		flushN(t, b, 5, 5)
		readN(t, b, 5, 10)

		err = b.Close()
		require.NoError(t, err)

		b2, err := NewMemoryBufferConfig().Build(buildContext, "test")
		require.NoError(t, err)

		readN(t, b2, 5, 0)
		readN(t, b2, 10, 10)
	})
}

func BenchmarkMemoryBuffer(b *testing.B) {
	buffer := newMemoryBuffer(b)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		e := entry.New()
		e.Record = "test log"
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			panicOnErr(buffer.Add(ctx, e))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		dst := make([]*entry.Entry, 1000)
		ctx := context.Background()
		for i := 0; i < b.N; {
			ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			c, n, err := buffer.ReadWait(ctx, dst)
			cancel()
			panicOnErr(err)
			i += n
			go func() {
				time.Sleep(50 * time.Millisecond)
				require.NoError(b, c.MarkAllAsFlushed())
			}()
		}
	}()

	wg.Wait()
}
