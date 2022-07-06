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

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type EmitFunc func(ctx context.Context, attrs *FileAttributes, token []byte)

// TODO rename this struct
type Input struct {
	*zap.SugaredLogger
	finder                  Finder
	captureFileName         bool
	captureFilePath         bool
	captureFileNameResolved bool
	captureFilePathResolved bool
	PollInterval            time.Duration
	SplitterConfig          helper.SplitterConfig
	MaxLogSize              int
	MaxConcurrentFiles      int
	SeenPaths               map[string]struct{}

	persister operator.Persister

	knownFiles    []*Reader
	queuedMatches []string
	maxBatchFiles int
	roller        roller

	startAtBeginning bool

	fingerprintSize int

	firstCheck bool
	wg         sync.WaitGroup
	cancel     context.CancelFunc

	emit EmitFunc
}

func (f *Input) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.firstCheck = true

	f.persister = persister

	// Load offsets from disk
	if err := f.loadLastPollFiles(ctx); err != nil {
		return fmt.Errorf("read known files from database: %w", err)
	}

	// Start polling goroutine
	f.startPoller(ctx)

	return nil
}

// Stop will stop the file monitoring process
func (f *Input) Stop() error {
	f.cancel()
	f.wg.Wait()
	f.roller.cleanup()
	for _, reader := range f.knownFiles {
		reader.Close()
	}
	f.knownFiles = nil
	f.cancel = nil
	return nil
}

// startPoller kicks off a goroutine that will poll the filesystem periodically,
// checking if there are new files or new logs in the watched files
func (f *Input) startPoller(ctx context.Context) {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		globTicker := time.NewTicker(f.PollInterval)
		defer globTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-globTicker.C:
			}

			f.poll(ctx)
		}
	}()
}

// poll checks all the watched paths for new entries
func (f *Input) poll(ctx context.Context) {
	f.maxBatchFiles = f.MaxConcurrentFiles / 2
	var matches []string
	if len(f.queuedMatches) > f.maxBatchFiles {
		matches, f.queuedMatches = f.queuedMatches[:f.maxBatchFiles], f.queuedMatches[f.maxBatchFiles:]
	} else {
		if len(f.queuedMatches) > 0 {
			matches, f.queuedMatches = f.queuedMatches, make([]string, 0)
		} else {
			// Increment the generation on all known readers
			// This is done here because the next generation is about to start
			for i := 0; i < len(f.knownFiles); i++ {
				f.knownFiles[i].generation++
			}

			// Get the list of paths on disk
			matches = f.finder.FindFiles()
			if f.firstCheck && len(matches) == 0 {
				f.Warnw("no files match the configured include patterns",
					"include", f.finder.Include,
					"exclude", f.finder.Exclude)
			} else if len(matches) > f.maxBatchFiles {
				matches, f.queuedMatches = matches[:f.maxBatchFiles], matches[f.maxBatchFiles:]
			}
		}
	}

	readers := f.makeReaders(matches)
	f.firstCheck = false

	// take care of files which disappeared from the pattern since the last poll cycle
	// this can mean either files which were removed, or rotated into a name not matching the pattern
	// we do this before reading existing files to ensure we emit older log lines before newer ones
	f.roller.readLostFiles(ctx, readers)

	var wg sync.WaitGroup
	for _, reader := range readers {
		wg.Add(1)
		go func(r *Reader) {
			defer wg.Done()
			r.ReadToEnd(ctx)
		}(reader)
	}
	wg.Wait()

	f.roller.roll(ctx, readers)
	f.saveCurrent(readers)
	f.syncLastPollFiles(ctx)
}

// makeReaders takes a list of paths, then creates readers from each of those paths,
// discarding any that have a duplicate fingerprint to other files that have already
// been read this polling interval
func (f *Input) makeReaders(filesPaths []string) []*Reader {
	// Open the files first to minimize the time between listing and opening
	files := make([]*os.File, 0, len(filesPaths))
	for _, path := range filesPaths {
		if _, ok := f.SeenPaths[path]; !ok {
			if f.startAtBeginning {
				f.Infow("Started watching file", "path", path)
			} else {
				f.Infow("Started watching file from end. To read preexisting logs, configure the argument 'start_at' to 'beginning'", "path", path)
			}
			f.SeenPaths[path] = struct{}{}
		}
		file, err := os.Open(path) // #nosec - operator must read in files defined by user
		if err != nil {
			f.Debugf("Failed to open file", zap.Error(err))
			continue
		}
		files = append(files, file)
	}

	// Get fingerprints for each file
	fps := make([]*Fingerprint, 0, len(files))
	for _, file := range files {
		fp, err := f.NewFingerprint(file)
		if err != nil {
			f.Errorw("Failed creating fingerprint", zap.Error(err))
			continue
		}
		fps = append(fps, fp)
	}

	// Exclude any empty fingerprints or duplicate fingerprints to avoid doubling up on copy-truncate files
OUTER:
	for i := 0; i < len(fps); i++ {
		fp := fps[i]
		if len(fp.FirstBytes) == 0 {
			if err := files[i].Close(); err != nil {
				f.Errorf("problem closing file", "file", files[i].Name())
			}
			// Empty file, don't read it until we can compare its fingerprint
			fps = append(fps[:i], fps[i+1:]...)
			files = append(files[:i], files[i+1:]...)
			i--
			continue
		}
		for j := i + 1; j < len(fps); j++ {
			fp2 := fps[j]
			if fp.StartsWith(fp2) || fp2.StartsWith(fp) {
				// Exclude
				if err := files[i].Close(); err != nil {
					f.Errorf("problem closing file", "file", files[i].Name())
				}
				fps = append(fps[:i], fps[i+1:]...)
				files = append(files[:i], files[i+1:]...)
				i--
				continue OUTER
			}
		}
	}

	readers := make([]*Reader, 0, len(fps))
	for i := 0; i < len(fps); i++ {
		reader, err := f.newReader(files[i], fps[i], f.firstCheck)
		if err != nil {
			f.Errorw("Failed to create reader", zap.Error(err))
			continue
		}
		readers = append(readers, reader)
	}

	return readers
}

// saveCurrent adds the readers from this polling interval to this list of
// known files, then increments the generation of all tracked old readers
// before clearing out readers that have existed for 3 generations.
func (f *Input) saveCurrent(readers []*Reader) {
	// Add readers from the current, completed poll interval to the list of known files
	f.knownFiles = append(f.knownFiles, readers...)

	// Clear out old readers. They are sorted such that they are oldest first,
	// so we can just find the first reader whose generation is less than our
	// max, and keep every reader after that
	for i := 0; i < len(f.knownFiles); i++ {
		reader := f.knownFiles[i]
		if reader.generation <= 3 {
			f.knownFiles = f.knownFiles[i:]
			break
		}
	}
}

func (f *Input) newReader(file *os.File, fp *Fingerprint, firstCheck bool) (*Reader, error) {
	// Check if the new path has the same fingerprint as an old path
	if oldReader, ok := f.findFingerprintMatch(fp); ok {
		newReader, err := oldReader.Copy(file)
		if err != nil {
			return nil, err
		}
		newReader.fileAttributes = f.resolveFileAttributes(file.Name())
		return newReader, nil
	}

	// If we don't match any previously known files, create a new reader from scratch
	splitter, err := f.getMultiline()
	if err != nil {
		return nil, err
	}
	newReader, err := f.NewReader(file.Name(), file, fp, splitter, f.emit)
	if err != nil {
		return nil, err
	}
	startAtBeginning := !firstCheck || f.startAtBeginning
	if err := newReader.InitializeOffset(startAtBeginning); err != nil {
		return nil, fmt.Errorf("initialize offset: %w", err)
	}
	return newReader, nil
}

func (f *Input) findFingerprintMatch(fp *Fingerprint) (*Reader, bool) {
	// Iterate backwards to match newest first
	for i := len(f.knownFiles) - 1; i >= 0; i-- {
		oldReader := f.knownFiles[i]
		if fp.StartsWith(oldReader.Fingerprint) {
			return oldReader, true
		}
	}
	return nil, false
}

const knownFilesKey = "knownFiles"

// syncLastPollFiles syncs the most recent set of files to the database
func (f *Input) syncLastPollFiles(ctx context.Context) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	// Encode the number of known files
	if err := enc.Encode(len(f.knownFiles)); err != nil {
		f.Errorw("Failed to encode known files", zap.Error(err))
		return
	}

	// Encode each known file
	for _, fileReader := range f.knownFiles {
		if err := enc.Encode(fileReader); err != nil {
			f.Errorw("Failed to encode known files", zap.Error(err))
		}
	}

	if err := f.persister.Set(ctx, knownFilesKey, buf.Bytes()); err != nil {
		f.Errorw("Failed to sync to database", zap.Error(err))
	}
}

// syncLastPollFiles loads the most recent set of files to the database
func (f *Input) loadLastPollFiles(ctx context.Context) error {
	encoded, err := f.persister.Get(ctx, knownFilesKey)
	if err != nil {
		return err
	}

	if encoded == nil {
		f.knownFiles = make([]*Reader, 0, 10)
		return nil
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))

	// Decode the number of entries
	var knownFileCount int
	if err := dec.Decode(&knownFileCount); err != nil {
		return fmt.Errorf("decoding file count: %w", err)
	}

	// Decode each of the known files
	f.knownFiles = make([]*Reader, 0, knownFileCount)
	for i := 0; i < knownFileCount; i++ {
		splitter, err := f.getMultiline()
		if err != nil {
			return err
		}
		newReader, err := f.NewReader("", nil, nil, splitter, f.emit)
		if err != nil {
			return err
		}
		if err = dec.Decode(newReader); err != nil {
			return err
		}
		f.knownFiles = append(f.knownFiles, newReader)
	}

	return nil
}

// getMultiline returns helper.Splitter structure and error eventually
func (f *Input) getMultiline() (*helper.Splitter, error) {
	return f.SplitterConfig.Build(false, f.MaxLogSize)
}
