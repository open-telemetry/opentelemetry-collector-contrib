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
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"testing"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func openBuffer(t testing.TB) *DiskBuffer {
	buffer := NewDiskBuffer(1 << 20)
	dir := testutil.NewTempDir(t)
	err := buffer.Open(dir, false)
	require.NoError(t, err)
	t.Cleanup(func() { buffer.Close() })
	return buffer
}

func compact(t testing.TB, b *DiskBuffer) {
	err := b.Compact()
	require.NoError(t, err)
}

func TestDiskBuffer(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
		writeN(t, b, 1, 0)
		readN(t, b, 1, 0)
	})

	t.Run("Write2Read1Read1", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
		writeN(t, b, 2, 0)
		readN(t, b, 1, 0)
		readN(t, b, 1, 1)
	})

	t.Run("Write20Read10Read10", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
		writeN(t, b, 20, 0)
		readN(t, b, 10, 0)
		readN(t, b, 10, 10)
	})

	t.Run("SingleReadWaitMultipleWrites", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
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
		b := openBuffer(t)
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
		b := openBuffer(t)
		writeN(t, b, 10, 0)
		readN(t, b, 10, 0)
		dst := make([]*entry.Entry, 10)
		_, n, err := b.Read(dst)
		require.NoError(t, err)
		require.Equal(t, 0, n)
	})

	t.Run("Write20Read10Read10Unfull", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
		writeN(t, b, 20, 0)
		readN(t, b, 10, 0)
		dst := make([]*entry.Entry, 20)
		_, n, err := b.Read(dst)
		require.NoError(t, err)
		require.Equal(t, 10, n)
	})

	t.Run("Write20Read10CompactRead10", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
		writeN(t, b, 20, 0)
		flushN(t, b, 10, 0)
		compact(t, b)
		readN(t, b, 10, 10)
	})

	t.Run("Write10Read2Flush2Read2Compact", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
		writeN(t, b, 10, 0)
		readN(t, b, 2, 0)
		flushN(t, b, 2, 2)
		readN(t, b, 2, 4)
		require.NoError(t, b.Compact())
	})

	t.Run("Write20Read10CloseRead20", func(t *testing.T) {
		t.Parallel()
		b := NewDiskBuffer(1 << 30)
		dir := testutil.NewTempDir(t)
		err := b.Open(dir, false)
		require.NoError(t, err)

		writeN(t, b, 20, 0)
		readN(t, b, 10, 0)
		err = b.Close()
		require.NoError(t, err)

		b2 := NewDiskBuffer(1 << 30)
		err = b2.Open(dir, false)
		require.NoError(t, err)
		readN(t, b2, 20, 0)
	})

	t.Run("Write20Flush10CloseRead20", func(t *testing.T) {
		t.Parallel()
		b := NewDiskBuffer(1 << 30)
		dir := testutil.NewTempDir(t)
		err := b.Open(dir, false)
		require.NoError(t, err)

		writeN(t, b, 20, 0)
		flushN(t, b, 10, 0)
		err = b.Close()
		require.NoError(t, err)

		b2 := NewDiskBuffer(1 << 30)
		err = b2.Open(dir, false)
		require.NoError(t, err)
		readN(t, b2, 10, 10)
	})

	t.Run("ReadWaitTimesOut", func(t *testing.T) {
		t.Parallel()
		b := openBuffer(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		dst := make([]*entry.Entry, 10)
		_, n, err := b.ReadWait(ctx, dst)
		require.NoError(t, err)
		require.Equal(t, 0, n)
	})

	t.Run("AddTimesOut", func(t *testing.T) {
		t.Parallel()
		b := NewDiskBuffer(100) // Enough space for 1, but not 2 entries
		dir := testutil.NewTempDir(t)
		err := b.Open(dir, false)
		require.NoError(t, err)

		// Add a first entry
		err = b.Add(context.Background(), entry.New())
		require.NoError(t, err)

		// Second entry should block and be cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		err = b.Add(ctx, entry.New())
		require.Error(t, err)
		cancel()

		// Read, flush, and compact
		dst := make([]*entry.Entry, 1)
		c, n, err := b.Read(dst)
		require.NoError(t, err)
		require.Equal(t, 1, n)
		require.NoError(t, c.MarkAllAsFlushed())
		require.NoError(t, b.Compact())

		// Now there should be space for another entry
		err = b.Add(context.Background(), entry.New())
		require.NoError(t, err)
	})

	t.Run("Write1kRandomFlushReadCompact", func(t *testing.T) {
		t.Parallel()
		rand.Seed(time.Now().Unix())
		seed := rand.Int63()
		t.Run(strconv.Itoa(int(seed)), func(t *testing.T) {
			t.Parallel()
			r := rand.New(rand.NewSource(seed))

			b := NewDiskBuffer(1 << 30)
			dir := testutil.NewTempDir(t)
			err := b.Open(dir, false)
			require.NoError(t, err)

			writes := 0
			reads := 0

			for i := 0; i < 1000; i++ {
				j := r.Int() % 1000
				switch {
				case j < 900:
					writeN(t, b, 1, writes)
					writes++
				case j < 990:
					readCount := (writes - reads) / 2
					c := readN(t, b, readCount, reads)
					if j%2 == 0 {
						require.NoError(t, c.MarkAllAsFlushed())
					}
					reads += readCount
				default:
					err := b.Compact()
					require.NoError(t, err)
				}
			}
		})
	})
}

func TestDiskBufferBuild(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		cfg := NewDiskBufferConfig()
		cfg.Path = testutil.NewTempDir(t)
		b, err := cfg.Build(testutil.NewBuildContext(t), "test")
		require.NoError(t, err)
		diskBuffer := b.(*DiskBuffer)
		require.Equal(t, diskBuffer.atEnd, false)
		require.Len(t, diskBuffer.entryAdded, 1)
		require.Equal(t, diskBuffer.maxBytes, int64(1<<32))
		require.Equal(t, diskBuffer.flushedBytes, int64(0))
		require.Len(t, diskBuffer.copyBuffer, 1<<16)
	})
}

func BenchmarkDiskBuffer(b *testing.B) {
	b.Run("NoSync", func(b *testing.B) {
		buffer := openBuffer(b)
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("Benchmark: %d\n", b.N)
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
				require.NoError(b, c.MarkAllAsFlushed())
			}
		}()

		wg.Wait()
	})

	b.Run("Sync", func(b *testing.B) {
		buffer := NewDiskBuffer(1 << 20)
		dir := testutil.NewTempDir(b)
		err := buffer.Open(dir, true)
		require.NoError(b, err)
		b.Cleanup(func() { buffer.Close() })
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("Benchmark: %d\n", b.N)
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
				require.NoError(b, c.MarkAllAsFlushed())
			}
		}()

		wg.Wait()
	})
}
