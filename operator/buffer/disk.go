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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"golang.org/x/sync/semaphore"
)

// DiskBufferConfig is a configuration struct for a DiskBuffer
type DiskBufferConfig struct {
	Type string `json:"type" yaml:"type"`

	// MaxSize is the maximum size in bytes of the data file on disk
	MaxSize helper.ByteSize `json:"max_size" yaml:"max_size"`

	// Path is a path to a directory which contains the data and metadata files
	Path string `json:"path" yaml:"path"`

	// Sync indicates whether to open the files with O_SYNC. If this is set to false,
	// in cases like power failures or unclean shutdowns, logs may be lost or the
	// database may become corrupted.
	Sync bool `json:"sync" yaml:"sync"`

	MaxChunkDelay helper.Duration `json:"max_delay"   yaml:"max_delay"`
	MaxChunkSize  uint            `json:"max_chunk_size" yaml:"max_chunk_size"`
}

// NewDiskBufferConfig creates a new default disk buffer config
func NewDiskBufferConfig() *DiskBufferConfig {
	return &DiskBufferConfig{
		Type:          "disk",
		MaxSize:       1 << 32, // 4GiB
		Sync:          true,
		MaxChunkDelay: helper.NewDuration(time.Second),
		MaxChunkSize:  1000,
	}
}

// Build creates a new Buffer from a DiskBufferConfig
func (c DiskBufferConfig) Build(context operator.BuildContext, _ string) (Buffer, error) {
	maxSize := c.MaxSize
	if maxSize == 0 {
		maxSize = 1 << 32
	}

	if c.Path == "" {
		return nil, fmt.Errorf("missing required field 'path'")
	}
	b := NewDiskBuffer(int64(maxSize))
	if err := b.Open(c.Path, c.Sync); err != nil {
		return nil, err
	}
	b.maxChunkSize = c.MaxChunkSize
	b.maxChunkDelay = c.MaxChunkDelay.Raw()
	return b, nil
}

// DiskBuffer is a buffer for storing entries on disk until they are flushed to their
// final destination.
type DiskBuffer struct {
	// Metadata holds information about the current state of the buffered entries
	metadata *Metadata

	// Data is the file that stores the buffered entries
	data *os.File
	sync.Mutex

	// atEnd indicates whether the data file descriptor is currently seeked to the
	// end. This helps save us seek calls.
	atEnd bool

	// entryAdded is a channel that is notified on every time an entry is added.
	// The integer sent down the channel is the new number of unread entries stored.
	// Readers using ReadWait will listen on this channel, and wait to read until
	// there are enough entries to fill its buffer.
	entryAdded chan int64

	maxBytes       int64
	flushedBytes   int64
	lastCompaction time.Time

	// readerLock ensures that there is only ever one reader listening to the
	// entryAdded channel at a time.
	readerLock sync.Mutex

	// diskSizeSemaphore is a semaphore that allows us to block once we've hit
	// the max disk size.
	diskSizeSemaphore *semaphore.Weighted

	// copyBuffer is a pre-allocated byte slice that is used during compaction
	copyBuffer []byte

	maxChunkDelay time.Duration
	maxChunkSize  uint
}

// NewDiskBuffer creates a new DiskBuffer
func NewDiskBuffer(maxDiskSize int64) *DiskBuffer {
	return &DiskBuffer{
		maxBytes:          maxDiskSize,
		entryAdded:        make(chan int64, 1),
		copyBuffer:        make([]byte, 1<<16),
		diskSizeSemaphore: semaphore.NewWeighted(maxDiskSize),
	}
}

// Open opens the disk buffer files from a database directory
func (d *DiskBuffer) Open(path string, sync bool) error {
	var err error
	dataPath := filepath.Join(path, "data")
	flags := os.O_CREATE | os.O_RDWR
	if sync {
		flags |= os.O_SYNC
	}
	if d.data, err = os.OpenFile(dataPath, flags, 0755); err != nil {
		return err
	}

	metadataPath := filepath.Join(path, "metadata")
	if d.metadata, err = OpenMetadata(metadataPath, sync); err != nil {
		return err
	}

	info, err := d.data.Stat()
	if err != nil {
		return err
	}

	if ok := d.diskSizeSemaphore.TryAcquire(info.Size()); !ok {
		return fmt.Errorf("current on-disk size is larger than max size")
	}

	// First, if there is a dead range from a previous incomplete compaction, delete it
	if err = d.deleteDeadRange(); err != nil {
		return err
	}

	// Compact on open
	if err = d.Compact(); err != nil {
		return err
	}

	// Once everything is compacted, we can safely reset all previously read, but
	// unflushed entries to unread
	d.metadata.unreadStartOffset = 0
	d.addUnreadCount(int64(len(d.metadata.read)))
	d.metadata.read = d.metadata.read[:0]
	return d.metadata.Sync()
}

// Close flushes the current metadata to disk, then closes the underlying files
func (d *DiskBuffer) Close() error {
	d.Lock()
	defer d.Unlock()

	if err := d.metadata.Close(); err != nil {
		return err
	}
	return d.data.Close()
}

// Add adds an entry to the buffer, blocking until it is either added or the context
// is cancelled.
func (d *DiskBuffer) Add(ctx context.Context, newEntry *entry.Entry) error {
	var buf bytes.Buffer
	var err error
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(newEntry); err != nil {
		return err
	}

	if err = d.diskSizeSemaphore.Acquire(ctx, int64(buf.Len())); err != nil {
		return err
	}

	d.Lock()
	defer d.Unlock()

	// Seek to end of the file if we're not there
	if err = d.seekToEnd(); err != nil {
		return err
	}

	if _, err = d.data.Write(buf.Bytes()); err != nil {
		return err
	}

	d.addUnreadCount(1)

	return nil
}

// addUnreadCount adds i to the unread count and notifies any callers of
// ReadWait that an entry has been added. The disk buffer lock must be held when
// calling this.
func (d *DiskBuffer) addUnreadCount(i int64) {
	d.metadata.unreadCount += i

	// Notify a reader that new entries have been added by either
	// sending on the channel, or updating the value in the channel
	select {
	case <-d.entryAdded:
		d.entryAdded <- d.metadata.unreadCount
	case d.entryAdded <- d.metadata.unreadCount:
	}
}

// ReadWait reads entries from the buffer, waiting until either there are enough entries in the
// buffer to fill dst or the context is cancelled. This amortizes the cost of reading from the
// disk. It returns a function that, when called, marks the read entries as flushed, the
// number of entries read, and an error.
func (d *DiskBuffer) ReadWait(ctx context.Context, dst []*entry.Entry) (Clearer, int, error) {
	d.readerLock.Lock()
	defer d.readerLock.Unlock()

	// Wait until the timeout is hit, or there are enough unread entries to fill the destination buffer
LOOP:
	for {
		select {
		case n := <-d.entryAdded:
			if n >= int64(len(dst)) {
				break LOOP
			}
		case <-ctx.Done():
			break LOOP
		}
	}

	return d.Read(dst)
}

// ReadChunk is a thin wrapper around ReadWait that simplifies the call at the expense of an extra allocation
func (d *DiskBuffer) ReadChunk(ctx context.Context) ([]*entry.Entry, Clearer, error) {
	entries := make([]*entry.Entry, d.maxChunkSize)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		ctx, cancel := context.WithTimeout(ctx, d.maxChunkDelay)
		defer cancel()
		flushFunc, n, err := d.ReadWait(ctx, entries)
		if n > 0 {
			return entries[:n], flushFunc, err
		}
	}
}

// Read copies entries from the disk into the destination buffer. It returns a function that,
// when called, marks the entries as flushed, the number of entries read, and an error.
func (d *DiskBuffer) Read(dst []*entry.Entry) (f Clearer, i int, err error) {
	d.Lock()
	defer d.Unlock()

	// Return fast if there are no unread entries
	if d.metadata.unreadCount == 0 {
		return d.newClearer(nil), 0, nil
	}

	// Seek to the start of the range of unread entries
	if err = d.seekToUnread(); err != nil {
		return nil, 0, fmt.Errorf("seek to unread: %s", err)
	}

	readCount := min(len(dst), int(d.metadata.unreadCount))
	newRead := make([]*readEntry, readCount)

	dec := json.NewDecoder(d.data)
	startOffset := d.metadata.unreadStartOffset
	for i := 0; i < readCount; i++ {
		// Decode an entry from the file
		var entry entry.Entry
		err := dec.Decode(&entry)
		if err != nil {
			return nil, 0, fmt.Errorf("decode: %s", err)
		}
		dst[i] = &entry

		// Calculate the end offset of the entry. We add one because
		// the decoder doesn't consume the newline at the end of every entry
		endOffset := d.metadata.unreadStartOffset + dec.InputOffset() + 1
		newRead[i] = &readEntry{
			startOffset: startOffset,
			length:      endOffset - startOffset,
		}

		// The start offset of the next entry is the end offset of the current
		startOffset = endOffset
	}

	// Set the offset for the next unread entry
	d.metadata.unreadStartOffset = startOffset

	// Keep track of the newly read entries
	d.metadata.read = append(d.metadata.read, newRead...)

	// Remove the read entries from the unread count
	d.addUnreadCount(-int64(readCount))

	return d.newClearer(newRead), readCount, nil
}

// newFlushFunc returns a function that marks read entries as flushed
func (d *DiskBuffer) newClearer(newRead []*readEntry) Clearer {
	return &diskClearer{
		buffer:      d,
		readEntries: newRead,
	}
}

type diskClearer struct {
	buffer      *DiskBuffer
	readEntries []*readEntry
}

func (dc *diskClearer) MarkAllAsFlushed() error {
	dc.buffer.Lock()
	for _, entry := range dc.readEntries {
		entry.flushed = true
		dc.buffer.flushedBytes += entry.length
	}
	dc.buffer.Unlock()
	return dc.buffer.checkCompact()
}

func (dc *diskClearer) MarkRangeAsFlushed(start, end uint) error {
	if int(end) > len(dc.readEntries) || int(start) > len(dc.readEntries) {
		return fmt.Errorf("invalid range")
	}

	dc.buffer.Lock()
	for _, entry := range dc.readEntries[start:end] {
		entry.flushed = true
		dc.buffer.flushedBytes += entry.length
	}
	dc.buffer.Unlock()
	return dc.buffer.checkCompact()
}

// checkCompact checks if a compaction should be performed, then kicks one off
func (d *DiskBuffer) checkCompact() error {
	d.Lock()
	switch {
	case d.flushedBytes > d.maxBytes/2:
		fallthrough
	case time.Since(d.lastCompaction) > 5*time.Second:
		d.Unlock()
		if err := d.Compact(); err != nil {
			return err
		}
	default:
		d.Unlock()
	}
	return nil
}

// Compact removes all flushed entries from disk
func (d *DiskBuffer) Compact() error {
	d.Lock()
	defer d.Unlock()
	defer func() { d.lastCompaction = time.Now() }()

	// So how does this work? The goal here is to remove all flushed entries from disk,
	// freeing up space for new entries. We do this by going through each entry that has
	// been read and checking if it has been flushed. If it has, we know that space on
	// disk is re-claimable, so we can move unflushed entries into its place.
	//
	// The tricky part is that we can't overwrite any data until we've both safely copied it
	// to its new location and written a copy of the metadata that describes where that data
	// is located on disk. This ensures that, if our process is killed mid-compaction, we will
	// always have a complete, uncorrupted database.
	//
	// We do this by maintaining a "dead range" during compation. The dead range is
	// effectively a range of bytes that can safely be deleted from disk just by shifting
	// everything that comes after it backwards in the file. Then, when we open the disk
	// buffer, the first thing we do is delete the dead range if it exists.
	//
	// To clear out flushed entries, we iterate over all the entries that have been read,
	// finding ranges of either flushed or unflushed entries. If we have a range of flushed
	// entries, we can expand the dead range to include the space those entries took on disk.
	// If we find a range of unflushed entries, we move them to the beginning of the dead range
	// and advance the start of the dead range to the end of the copied bytes.
	//
	// Once we iterate through all the read entries, we should be left with a dead range
	// that's located right before the start of the unread entries. Since we know none of the
	// unread entries need be flushed, we can simply bubble the dead range through the unread
	// entries, then truncate the dead range from the end of the file once we're done.
	//
	// The most important part here is to sync the metadata to disk before overwriting any
	// data. That way, at startup, we know where the dead zone is in the file so we can
	// safely delete it without deleting any live data.
	//
	// Example:
	// (f = flushed byte, r = read byte, u = unread byte, lowercase = dead range)
	//
	// FFFFRRRRFFRRRRRRRUUUUUUUUU // start of compaction
	// ffffRRRRFFRRRRRRRUUUUUUUUU // mark the first flushed range as unread
	// RRRRrrrrFFRRRRRRRUUUUUUUUU // move the read range to the beginning of the dead range
	// RRRRrrrrffRRRRRRRUUUUUUUUU // expand the dead range to include the flushed range
	// RRRRRRRRRRrrrrrrRUUUUUUUUU // move the portion of the next read range that fits into the dead range
	// RRRRRRRRRRRrrrrrrUUUUUUUUU // move the remainder of the read range to start of the dead range
	// RRRRRRRRRRRUUUUUUuuuuuuUUU // move the unread entries that fit into the dead range
	// RRRRRRRRRRRUUUUUUUUUuuuuuu // move the remainder of the unread entries into the dead range
	// RRRRRRRRRRRUUUUUUUUU       // truncate the file to remove the dead range

	m := d.metadata
	if m.deadRangeLength != 0 {
		return fmt.Errorf("cannot compact the disk buffer before removing the dead range")
	}

	for start := 0; start < len(m.read); {
		if m.read[start].flushed {
			// Find the end index of the range of flushed entries
			end := start + 1
			for ; end < len(m.read); end++ {
				if !m.read[end].flushed {
					break
				}
			}

			// Expand the dead range
			if err := d.markFlushedRangeDead(start, end); err != nil {
				return err
			}
		} else {
			// Find the end index of the range of unflushed entries
			end := start + 1
			for ; end < len(m.read); end++ {
				if m.read[end].flushed {
					break
				}
			}

			// Slide the unread range left, to the start of the dead range
			if err := d.shiftUnreadRange(start, end); err != nil {
				return err
			}

			// Update i
			start = end
		}
	}

	// Bubble the dead space through the unflushed entries, then truncate
	return d.deleteDeadRange()
}

func (d *DiskBuffer) markFlushedRangeDead(start, end int) error {
	m := d.metadata
	// Expand the dead range
	rangeSize := onDiskSize(m.read[start:end])
	m.deadRangeLength += rangeSize
	d.flushedBytes -= rangeSize

	// Update the effective offsets if the dead range is removed
	for _, entry := range m.read[end:] {
		entry.startOffset -= rangeSize
	}

	// Update the effective unreadStartOffset if the dead range is removed
	m.unreadStartOffset -= rangeSize

	// Delete the flushed range from metadata
	m.read = append(m.read[:start], m.read[end:]...)

	// Sync to disk
	return d.metadata.Sync()
}

func (d *DiskBuffer) shiftUnreadRange(start, end int) error {
	m := d.metadata
	// If there is no dead range, no need to move unflushed entries
	rangeSize := onDiskSize(m.read[start:end])
	if m.deadRangeLength == 0 {
		m.deadRangeStart += rangeSize
		return nil
	}

	// Slide the range left, syncing dead range after every chunk
	for bytesMoved := 0; bytesMoved < int(rangeSize); {
		remainingBytes := int(rangeSize) - bytesMoved
		chunkSize := min(int(m.deadRangeLength), remainingBytes)

		// Move the chunk to the beginning of the dead space
		_, err := d.moveRange(
			m.deadRangeStart,
			m.deadRangeLength,
			m.read[start].startOffset+int64(bytesMoved)+m.deadRangeLength,
			int64(chunkSize),
		)
		if err != nil {
			return err
		}

		// Update the offset of the dead space
		m.deadRangeStart += int64(chunkSize)
		bytesMoved += chunkSize

		// Sync to disk all at once
		if err = d.metadata.Sync(); err != nil {
			return err
		}
	}

	return nil
}

// deleteDeadRange moves the dead range to the end of the file, chunk by chunk,
// so that if it is interrupted, it can just be continued at next startup.
func (d *DiskBuffer) deleteDeadRange() error {
	// Exit fast if there is no dead range
	if d.metadata.deadRangeLength == 0 {
		if d.metadata.deadRangeStart != 0 {
			return d.metadata.setDeadRange(0, 0)
		}
		return nil
	}

	for {
		// Replace the range with the proceeding range of bytes
		start := d.metadata.deadRangeStart
		length := d.metadata.deadRangeLength
		n, err := d.moveRange(
			start,
			length,
			start+length,
			length,
		)
		if err != nil {
			return err
		}

		// Update the dead range, writing to disk
		if err = d.metadata.setDeadRange(start+length, length); err != nil {
			return err
		}

		if int64(n) < d.metadata.deadRangeLength {
			// We're at the end of the file
			break
		}
	}

	info, err := d.data.Stat()
	if err != nil {
		return err
	}

	// Truncate the extra space at the end of the file
	if err = d.data.Truncate(info.Size() - d.metadata.deadRangeLength); err != nil {
		return err
	}

	d.diskSizeSemaphore.Release(d.metadata.deadRangeLength)

	if err = d.metadata.setDeadRange(0, 0); err != nil {
		return err
	}

	return nil
}

// moveRange moves from length2 bytes starting from start2 into the space from start1
// to start1+length1
func (d *DiskBuffer) moveRange(start1, length1, start2, length2 int64) (int, error) {
	if length2 > length1 {
		return 0, fmt.Errorf("cannot move a range into a space smaller than itself")
	}

	readPosition := start2
	writePosition := start1
	bytesRead := 0

	rd := io.LimitReader(d.data, length2)

	eof := false
	for !eof {
		// Seek to last read position
		if err := d.seekTo(readPosition); err != nil {
			return 0, err
		}

		// Read a chunk
		n, err := rd.Read(d.copyBuffer)
		if err != nil {
			if err != io.EOF {
				return 0, err
			}
			eof = true
		}
		readPosition += int64(n)
		bytesRead += n

		// Write the chunk back into a free region
		_, err = d.data.WriteAt(d.copyBuffer[:n], writePosition)
		if err != nil {
			return 0, err
		}
		writePosition += int64(n)
	}

	return bytesRead, nil
}

// seekToEnd seeks the data file descriptor to the end of the
// file, but only if we're not already at the end.
func (d *DiskBuffer) seekToEnd() error {
	if !d.atEnd {
		if _, err := d.data.Seek(0, 2); err != nil {
			return err
		}
		d.atEnd = true
	}
	return nil
}

// seekToUnread seeks the data file descriptor to the beginning
// of the first unread message
func (d *DiskBuffer) seekToUnread() error {
	_, err := d.data.Seek(d.metadata.unreadStartOffset, 0)
	d.atEnd = false
	return err
}

// seekTo seeks the data file descriptor to the given offset.
// This is used rather than (*os.File).Seek() because it also sets
// atEnd correctly.
func (d *DiskBuffer) seekTo(pos int64) error {
	// Seek to last read position
	_, err := d.data.Seek(pos, 0)
	d.atEnd = false
	return err
}

// min returns the minimum of two ints
func min(first, second int) int {
	m := first
	if second < first {
		m = second
	}
	return m
}
