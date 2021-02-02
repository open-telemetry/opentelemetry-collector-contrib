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
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/database"
	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"go.etcd.io/bbolt"
	"golang.org/x/sync/semaphore"
)

// MemoryBufferConfig holds the configuration for a memory buffer
type MemoryBufferConfig struct {
	Type          string          `json:"type"        yaml:"type"`
	MaxEntries    int             `json:"max_entries" yaml:"max_entries"`
	MaxChunkDelay helper.Duration `json:"max_delay"   yaml:"max_delay"`
	MaxChunkSize  uint            `json:"max_chunk_size" yaml:"max_chunk_size"`
}

// NewMemoryBufferConfig creates a new default MemoryBufferConfig
func NewMemoryBufferConfig() *MemoryBufferConfig {
	return &MemoryBufferConfig{
		Type:          "memory",
		MaxEntries:    1 << 20,
		MaxChunkDelay: helper.NewDuration(time.Second),
		MaxChunkSize:  1000,
	}
}

// Build builds a MemoryBufferConfig into a Buffer, loading any entries that were previously unflushed
// back into memory
func (c MemoryBufferConfig) Build(context operator.BuildContext, pluginID string) (Buffer, error) {
	mb := &MemoryBuffer{
		db:            context.Database,
		pluginID:      pluginID,
		buf:           make(chan *entry.Entry, c.MaxEntries),
		sem:           semaphore.NewWeighted(int64(c.MaxEntries)),
		inFlight:      make(map[uint64]*entry.Entry, c.MaxEntries),
		maxChunkDelay: c.MaxChunkDelay.Raw(),
		maxChunkSize:  c.MaxChunkSize,
	}
	if err := mb.loadFromDB(); err != nil {
		return nil, err
	}

	return mb, nil
}

// MemoryBuffer is a buffer that holds all entries in memory until Close() is called,
// at which point it saves the entries into a database. It provides no guarantees about
// lost entries if shut down uncleanly.
type MemoryBuffer struct {
	db            database.Database
	pluginID      string
	buf           chan *entry.Entry
	inFlight      map[uint64]*entry.Entry
	inFlightMux   sync.Mutex
	entryID       uint64
	sem           *semaphore.Weighted
	maxChunkDelay time.Duration
	maxChunkSize  uint
}

// Add inserts an entry into the memory database, blocking until there is space
func (m *MemoryBuffer) Add(ctx context.Context, e *entry.Entry) error {
	if err := m.sem.Acquire(ctx, 1); err != nil {
		return err
	}

	m.buf <- e
	return nil
}

// Read reads entries until either there are no entries left in the buffer
// or the destination slice is full. The returned function must be called
// once the entries are flushed to remove them from the memory buffer.
func (m *MemoryBuffer) Read(dst []*entry.Entry) (Clearer, int, error) {
	inFlight := make([]uint64, len(dst))
	i := 0
	for ; i < len(dst); i++ {
		select {
		case e := <-m.buf:
			dst[i] = e
			id := atomic.AddUint64(&m.entryID, 1)
			m.inFlightMux.Lock()
			m.inFlight[id] = e
			m.inFlightMux.Unlock()
			inFlight[i] = id
		default:
			return m.newClearer(inFlight[:i]), i, nil
		}
	}

	return m.newClearer(inFlight[:i]), i, nil
}

// ReadChunk is a thin wrapper around ReadWait that simplifies the call at the expense of an extra allocation
func (m *MemoryBuffer) ReadChunk(ctx context.Context) ([]*entry.Entry, Clearer, error) {
	entries := make([]*entry.Entry, m.maxChunkSize)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		ctx, cancel := context.WithTimeout(ctx, m.maxChunkDelay)
		defer cancel()
		flushFunc, n, err := m.ReadWait(ctx, entries)
		if n > 0 {
			return entries[:n], flushFunc, err
		}
	}
}

// ReadWait reads entries until either the destination slice is full, or the context passed to it
// is cancelled. The returned function must be called once the entries are flushed to remove them
// from the memory buffer
func (m *MemoryBuffer) ReadWait(ctx context.Context, dst []*entry.Entry) (Clearer, int, error) {
	inFlightIDs := make([]uint64, len(dst))
	i := 0
	for ; i < len(dst); i++ {
		select {
		case e := <-m.buf:
			dst[i] = e
			id := atomic.AddUint64(&m.entryID, 1)
			m.inFlightMux.Lock()
			m.inFlight[id] = e
			m.inFlightMux.Unlock()
			inFlightIDs[i] = id
		case <-ctx.Done():
			return m.newClearer(inFlightIDs[:i]), i, nil
		}
	}

	return m.newClearer(inFlightIDs[:i]), i, nil
}

type memoryClearer struct {
	buffer *MemoryBuffer
	ids    []uint64
}

func (mc *memoryClearer) MarkAllAsFlushed() error {
	mc.buffer.inFlightMux.Lock()
	for _, id := range mc.ids {
		delete(mc.buffer.inFlight, id)
	}
	mc.buffer.inFlightMux.Unlock()
	mc.buffer.sem.Release(int64(len(mc.ids)))
	return nil
}

func (mc *memoryClearer) MarkRangeAsFlushed(start, end uint) error {
	if int(end) > len(mc.ids) || int(start) > len(mc.ids) {
		return fmt.Errorf("invalid range")
	}

	mc.buffer.inFlightMux.Lock()
	for _, id := range mc.ids[start:end] {
		delete(mc.buffer.inFlight, id)
	}
	mc.buffer.inFlightMux.Unlock()
	mc.buffer.sem.Release(int64(len(mc.ids)))
	return nil
}

// newFlushFunc returns a function that will remove the entries identified by `ids` from the buffer
func (m *MemoryBuffer) newClearer(ids []uint64) Clearer {
	return &memoryClearer{
		buffer: m,
		ids:    ids,
	}
}

// Close closes the memory buffer, saving all entries currently in the memory buffer to the
// agent's database.
func (m *MemoryBuffer) Close() error {
	m.inFlightMux.Lock()
	defer m.inFlightMux.Unlock()
	return m.db.Update(func(tx *bbolt.Tx) error {
		memBufBucket, err := tx.CreateBucketIfNotExists([]byte("memory_buffer"))
		if err != nil {
			return err
		}

		b, err := memBufBucket.CreateBucketIfNotExists([]byte(m.pluginID))
		if err != nil {
			return err
		}

		for k, v := range m.inFlight {
			if err := putKeyValue(b, k, v); err != nil {
				return err
			}
		}

		for {
			select {
			case e := <-m.buf:
				m.entryID++
				if err := putKeyValue(b, m.entryID, e); err != nil {
					return err
				}
			default:
				return nil
			}
		}
	})
}

func putKeyValue(b *bbolt.Bucket, k uint64, v *entry.Entry) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	key := [8]byte{}

	binary.LittleEndian.PutUint64(key[:], k)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return b.Put(key[:], buf.Bytes())
}

// loadFromDB loads any entries saved to the database previously into the memory buffer,
// allowing them to be flushed
func (m *MemoryBuffer) loadFromDB() error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		memBufBucket := tx.Bucket([]byte("memory_buffer"))
		if memBufBucket == nil {
			return nil
		}

		b := memBufBucket.Bucket([]byte(m.pluginID))
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			if ok := m.sem.TryAcquire(1); !ok {
				return fmt.Errorf("max_entries is smaller than the number of entries stored in the database")
			}

			dec := json.NewDecoder(bytes.NewReader(v))
			var e entry.Entry
			if err := dec.Decode(&e); err != nil {
				return err
			}

			select {
			case m.buf <- &e:
				return nil
			default:
				return fmt.Errorf("max_entries is smaller than the number of entries stored in the database")
			}
		})
	})
}
