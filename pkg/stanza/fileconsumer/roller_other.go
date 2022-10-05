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

//go:build !windows
// +build !windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"sync"
)

type detectLostFiles struct {
	oldReaders []*Reader
}

func newRoller() roller {
	return &detectLostFiles{[]*Reader{}}
}

func (r *detectLostFiles) readLostFiles(ctx context.Context, readers []*Reader) {
	// Detect files that have been rotated out of matching pattern
	lostReaders := make([]*Reader, 0, len(r.oldReaders))
OUTER:
	for _, oldReader := range r.oldReaders {
		for _, reader := range readers {
			if reader.Fingerprint.StartsWith(oldReader.Fingerprint) {
				continue OUTER
			}
		}
		lostReaders = append(lostReaders, oldReader)
	}

	var lostWG sync.WaitGroup
	for _, reader := range lostReaders {
		lostWG.Add(1)
		go func(r *Reader) {
			defer lostWG.Done()
			r.ReadToEnd(ctx)
		}(reader)
	}
	lostWG.Wait()
}

func (r *detectLostFiles) roll(ctx context.Context, readers []*Reader) {
	for _, reader := range r.oldReaders {
		reader.Close()
	}

	r.oldReaders = readers
}

func (r *detectLostFiles) cleanup() {
	for _, reader := range r.oldReaders {
		reader.Close()
	}
}
