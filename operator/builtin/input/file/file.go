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

package file

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"
)

// InputOperator is an operator that monitors files for entries
type InputOperator struct {
	helper.InputOperator

	Include            []string
	Exclude            []string
	FilePathField      entry.Field
	FileNameField      entry.Field
	PollInterval       time.Duration
	SplitFunc          bufio.SplitFunc
	MaxLogSize         int
	MaxConcurrentFiles int
	SeenPaths          map[string]struct{}

	persist helper.Persister

	knownFiles    []*Reader
	queuedMatches []string

	startAtBeginning bool

	fingerprintSize int

	encoding encoding.Encoding

	wg         sync.WaitGroup
	firstCheck bool
	cancel     context.CancelFunc
}

// Start will start the file monitoring process
func (f *InputOperator) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.firstCheck = true

	// Load offsets from disk
	if err := f.loadLastPollFiles(); err != nil {
		return fmt.Errorf("read known files from database: %s", err)
	}

	// Start polling goroutine
	f.startPoller(ctx)

	return nil
}

// Stop will stop the file monitoring process
func (f *InputOperator) Stop() error {
	f.cancel()
	f.wg.Wait()
	f.knownFiles = nil
	f.cancel = nil
	return nil
}

// startPoller kicks off a goroutine that will poll the filesystem periodically,
// checking if there are new files or new logs in the watched files
func (f *InputOperator) startPoller(ctx context.Context) {
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
func (f *InputOperator) poll(ctx context.Context) {
	var matches []string
	if len(f.queuedMatches) > f.MaxConcurrentFiles {
		matches, f.queuedMatches = f.queuedMatches[:f.MaxConcurrentFiles], f.queuedMatches[f.MaxConcurrentFiles:]
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
			matches = getMatches(f.Include, f.Exclude)
			if f.firstCheck && len(matches) == 0 {
				f.Warnw("no files match the configured include patterns", "include", f.Include)
			} else if len(matches) > f.MaxConcurrentFiles {
				matches, f.queuedMatches = matches[:f.MaxConcurrentFiles], matches[f.MaxConcurrentFiles:]
			}
		}
	}

	// Open the files first to minimize the time between listing and opening
	files := make([]*os.File, 0, len(matches))
	for _, path := range matches {
		if _, ok := f.SeenPaths[path]; !ok {
			if f.startAtBeginning {
				f.Infow("Started watching file", "path", path)
			} else {
				f.Infow("Started watching file from end. To read preexisting logs, configure the argument 'start_at' to 'beginning'", "path", path)
			}
			f.SeenPaths[path] = struct{}{}
		}
		file, err := os.Open(path)
		if err != nil {
			f.Errorw("Failed to open file", zap.Error(err))
			continue
		}
		files = append(files, file)
	}

	readers := f.makeReaders(files)
	f.firstCheck = false

	var wg sync.WaitGroup
	for _, reader := range readers {
		wg.Add(1)
		go func(r *Reader) {
			defer wg.Done()
			r.ReadToEnd(ctx)
		}(reader)
	}

	// Wait until all the reader goroutines are finished
	wg.Wait()

	// Close all files
	for _, file := range files {
		file.Close()
	}

	f.saveCurrent(readers)
	f.syncLastPollFiles()
}

// getMatches gets a list of paths given an array of glob patterns to include and exclude
func getMatches(includes, excludes []string) []string {
	all := make([]string, 0, len(includes))
	for _, include := range includes {
		matches, _ := filepath.Glob(include) // compile error checked in build
	INCLUDE:
		for _, match := range matches {
			for _, exclude := range excludes {
				if itMatches, _ := filepath.Match(exclude, match); itMatches {
					continue INCLUDE
				}
			}

			for _, existing := range all {
				if existing == match {
					continue INCLUDE
				}
			}

			all = append(all, match)
		}
	}

	return all
}

// makeReaders takes a list of paths, then creates readers from each of those paths,
// discarding any that have a duplicate fingerprint to other files that have already
// been read this polling interval
func (f *InputOperator) makeReaders(files []*os.File) []*Reader {
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

	// Make a copy of the files so we don't modify the original
	filesCopy := make([]*os.File, len(files))
	copy(filesCopy, files)

	// Exclude any empty fingerprints or duplicate fingerprints to avoid doubling up on copy-truncate files
OUTER:
	for i := 0; i < len(fps); {
		fp := fps[i]
		if len(fp.FirstBytes) == 0 {
			// Empty file, don't read it until we can compare its fingerprint
			fps = append(fps[:i], fps[i+1:]...)
			filesCopy = append(filesCopy[:i], filesCopy[i+1:]...)
		}

		for j := 0; j < len(fps); j++ {
			if i == j {
				// Skip checking itself
				continue
			}

			fp2 := fps[j]
			if fp.StartsWith(fp2) || fp2.StartsWith(fp) {
				// Exclude
				fps = append(fps[:i], fps[i+1:]...)
				filesCopy = append(filesCopy[:i], filesCopy[i+1:]...)
				continue OUTER
			}
		}
		i++
	}

	readers := make([]*Reader, 0, len(fps))
	for i := 0; i < len(fps); i++ {
		reader, err := f.newReader(filesCopy[i], fps[i], f.firstCheck)
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
func (f *InputOperator) saveCurrent(readers []*Reader) {
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

func (f *InputOperator) newReader(file *os.File, fp *Fingerprint, firstCheck bool) (*Reader, error) {
	// Check if the new path has the same fingerprint as an old path
	if oldReader, ok := f.findFingerprintMatch(fp); ok {
		newReader, err := oldReader.Copy(file)
		if err != nil {
			return nil, err
		}
		newReader.Path = file.Name()
		return newReader, nil
	}

	// If we don't match any previously known files, create a new reader from scratch
	newReader, err := f.NewReader(file.Name(), file, fp)
	if err != nil {
		return nil, err
	}
	startAtBeginning := !firstCheck || f.startAtBeginning
	if err := newReader.InitializeOffset(startAtBeginning); err != nil {
		return nil, fmt.Errorf("initialize offset: %s", err)
	}
	return newReader, nil
}

func (f *InputOperator) findFingerprintMatch(fp *Fingerprint) (*Reader, bool) {
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
func (f *InputOperator) syncLastPollFiles() {
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

	f.persist.Set(knownFilesKey, buf.Bytes())
	if err := f.persist.Sync(); err != nil {
		f.Errorw("Failed to sync to database", zap.Error(err))
	}
}

// syncLastPollFiles loads the most recent set of files to the database
func (f *InputOperator) loadLastPollFiles() error {
	err := f.persist.Load()
	if err != nil {
		return err
	}

	encoded := f.persist.Get(knownFilesKey)
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
		newReader, err := f.NewReader("", nil, nil)
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
