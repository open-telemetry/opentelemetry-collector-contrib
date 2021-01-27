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
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Metadata is a representation of the on-disk metadata file. It contains
// information about the layout, location, and flushed status of entries
// stored in the data file
type Metadata struct {
	// File is a handle to the on-disk metadata store
	//
	// The layout of the file is as follows:
	// - 8 byte DatabaseVersion as LittleEndian int64
	// - 8 byte DeadRangeStartOffset as LittleEndian int64
	// - 8 byte DeadRangeLength as LittleEndian int64
	// - 8 byte UnreadStartOffset as LittleEndian int64
	// - 8 byte UnreadCount as LittleEndian int64
	// - 8 byte ReadCount as LittleEndian int64
	// - Repeated ReadCount times:
	//     - 1 byte Flushed bool LittleEndian
	//     - 8 byte Length as LittleEndian int64
	//     - 8 byte StartOffset as LittleEndian int64
	file *os.File

	// read is the collection of entries that have been read
	read []*readEntry

	// unreadStartOffset is the offset on disk where the contiguous
	// range of unread entries start
	unreadStartOffset int64

	// unreadCount is the number of unread entries on disk
	unreadCount int64

	// deadRangeStart is file offset of the beginning of the dead range.
	// The dead range is a range of the file that contains unused information
	// and should only exist during a compaction. If this exists on startup,
	// it should be removed as part of the startup compaction.
	deadRangeStart int64

	// deadRangeLength is the length of the dead range
	deadRangeLength int64
}

// OpenMetadata opens and parses the metadata
func OpenMetadata(path string, sync bool) (*Metadata, error) {
	m := &Metadata{}

	var err error
	flags := os.O_CREATE | os.O_RDWR
	if sync {
		flags |= os.O_SYNC
	}
	if m.file, err = os.OpenFile(path, flags, 0755); err != nil {
		return &Metadata{}, err
	}

	info, err := m.file.Stat()
	if err != nil {
		return &Metadata{}, err
	}

	if info.Size() > 0 {
		err = m.UnmarshalBinary(m.file)
		if err != nil {
			return &Metadata{}, fmt.Errorf("read metadata file: %s", err)
		}
	} else {
		m.read = make([]*readEntry, 0, 1000)
	}

	return m, nil
}

// Sync persists the metadata to disk
func (m *Metadata) Sync() error {
	// Serialize to a buffer first so we write all at once
	var buf bytes.Buffer
	if err := m.MarshalBinary(&buf); err != nil {
		return err
	}

	// Write the whole thing to the file
	n, err := m.file.WriteAt(buf.Bytes(), 0)
	if err != nil {
		return err
	}

	// Since our on-disk format for metadata self-describes length,
	// it's okay to truncate as a separate operation because an un-truncated
	// file is still readable
	err = m.file.Truncate(int64(n))
	if err != nil {
		return err
	}

	return nil
}

// Close syncs metadata to disk and closes the underlying file descriptor
func (m *Metadata) Close() error {
	err := m.Sync()
	if err != nil {
		return err
	}
	m.file.Close()
	return nil
}

// setDeadRange sets the dead range start and length, then persists it to disk
// without rewriting the whole file
func (m *Metadata) setDeadRange(start, length int64) error {
	m.deadRangeStart = start
	m.deadRangeLength = length
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, m.deadRangeStart); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.LittleEndian, m.deadRangeLength); err != nil {
		return err
	}
	_, err := m.file.WriteAt(buf.Bytes(), 8)
	return err
}

// MarshalBinary marshals a metadata struct to a binary stream
func (m *Metadata) MarshalBinary(wr io.Writer) (err error) {
	// Version (currently unused)
	if err = binary.Write(wr, binary.LittleEndian, int64(1)); err != nil {
		return
	}

	// Dead Range info
	if err = binary.Write(wr, binary.LittleEndian, m.deadRangeStart); err != nil {
		return
	}
	if err = binary.Write(wr, binary.LittleEndian, m.deadRangeLength); err != nil {
		return
	}

	// Unread range info
	if err = binary.Write(wr, binary.LittleEndian, m.unreadStartOffset); err != nil {
		return
	}
	if err = binary.Write(wr, binary.LittleEndian, m.unreadCount); err != nil {
		return
	}

	// Read entries offsets
	if err = binary.Write(wr, binary.LittleEndian, int64(len(m.read))); err != nil {
		return
	}
	for _, readEntry := range m.read {
		if err = readEntry.MarshalBinary(wr); err != nil {
			return
		}
	}
	return nil
}

// UnmarshalBinary unmarshals metadata from a binary stream (usually a file)
func (m *Metadata) UnmarshalBinary(r io.Reader) error {
	// Read version
	var version int64
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return fmt.Errorf("failed to read version: %s", err)
	}

	// Read dead range
	if err := binary.Read(r, binary.LittleEndian, &m.deadRangeStart); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &m.deadRangeLength); err != nil {
		return err
	}

	// Read unread info
	if err := binary.Read(r, binary.LittleEndian, &m.unreadStartOffset); err != nil {
		return fmt.Errorf("read unread start offset: %s", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &m.unreadCount); err != nil {
		return fmt.Errorf("read contiguous count: %s", err)
	}

	// Read read info
	var readCount int64
	if err := binary.Read(r, binary.LittleEndian, &readCount); err != nil {
		return fmt.Errorf("read read count: %s", err)
	}
	m.read = make([]*readEntry, readCount)
	for i := 0; i < int(readCount); i++ {
		newEntry := &readEntry{}
		if err := newEntry.UnmarshalBinary(r); err != nil {
			return err
		}

		m.read[i] = newEntry
	}

	return nil
}

// readEntry is a struct holding metadata about read entries
type readEntry struct {
	// A flushed entry is one that has been flushed and is ready
	// to be removed from disk
	flushed bool

	// The number of bytes the entry takes on disk
	length int64

	// The offset in the file where the entry starts
	startOffset int64
}

// MarshalBinary marshals a readEntry struct to a binary stream
func (re readEntry) MarshalBinary(wr io.Writer) error {
	if err := binary.Write(wr, binary.LittleEndian, re.flushed); err != nil {
		return err
	}
	if err := binary.Write(wr, binary.LittleEndian, re.length); err != nil {
		return err
	}
	if err := binary.Write(wr, binary.LittleEndian, re.startOffset); err != nil {
		return err
	}
	return nil
}

// UnmarshalBinary unmarshals a binary stream into a readEntry struct
func (re *readEntry) UnmarshalBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &re.flushed); err != nil {
		return fmt.Errorf("read disk entry flushed: %s", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &re.length); err != nil {
		return fmt.Errorf("read disk entry length: %s", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &re.startOffset); err != nil {
		return fmt.Errorf("read disk entry start offset: %s", err)
	}
	return nil
}

// onDiskSize calculates the size in bytes on disk for a contiguous
// range of diskEntries
func onDiskSize(entries []*readEntry) int64 {
	if len(entries) == 0 {
		return 0
	}

	last := entries[len(entries)-1]
	return last.startOffset + last.length - entries[0].startOffset
}
