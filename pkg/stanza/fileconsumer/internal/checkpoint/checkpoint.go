// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const knownFilesKey = "knownFiles"

// Checkpoint Retention Policy:
// -----------------------------------
// Checkpoints for file offsets are not kept forever. Instead, a ring buffer mechanism is used (see tracker.go),
// controlled by the 'pollsToArchive' parameter. This limits the number of archived checkpoint sets (e.g., knownFiles0, knownFiles1, ...).
// When the buffer wraps, old checkpoints are overwritten, bounding storage usage.
//
// Tradeoff: If a file is inactive for longer than the buffer covers, its checkpoint may be lost, and if updated later,
// the file will be reprocessed from the beginning, causing data duplication. Set 'pollsToArchive' high enough to cover
// the maximum expected inactivity period for files that may be updated again. For files that are truly finalized, their
// checkpoints can be removed immediately if desired.
// -----------------------------------

// Save syncs the most recent set of files to the database
func Save(ctx context.Context, persister operator.Persister, rmds []*reader.Metadata) error {
	return SaveKey(ctx, persister, rmds, knownFilesKey)
}

func SaveKey(ctx context.Context, persister operator.Persister, rmds []*reader.Metadata, key string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	// Encode the number of known files
	if err := enc.Encode(len(rmds)); err != nil {
		return fmt.Errorf("encode num files: %w", err)
	}

	var errs []error
	// Encode each known file
	for _, rmd := range rmds {
		if err := enc.Encode(rmd); err != nil {
			errs = append(errs, fmt.Errorf("encode metadata: %w", err))
		}
	}

	if err := persister.Set(ctx, key, buf.Bytes()); err != nil {
		errs = append(errs, fmt.Errorf("persist known files: %w", err))
	}

	return errors.Join(errs...)
}

// Load loads the most recent set of files to the database
func Load(ctx context.Context, persister operator.Persister) ([]*reader.Metadata, error) {
	encoded, err := persister.Get(ctx, knownFilesKey)
	if err != nil {
		return nil, err
	}

	if encoded == nil {
		return []*reader.Metadata{}, nil
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))

	// Decode the number of entries
	var knownFileCount int
	if err = dec.Decode(&knownFileCount); err != nil {
		return nil, fmt.Errorf("decoding file count: %w", err)
	}

	// Decode each of the known files
	var errs []error
	rmds := make([]*reader.Metadata, 0, knownFileCount)
	for i := 0; i < knownFileCount; i++ {
		rmd := new(reader.Metadata)
		if err = dec.Decode(rmd); err != nil {
			return nil, err
		}
		if rmd.FileAttributes == nil {
			rmd.FileAttributes = map[string]any{}
		}
		// This reader won't be used for anything other than metadata reference, so just wrap the metadata
		rmds = append(rmds, rmd)
	}
	return rmds, errors.Join(errs...)
}

func LoadKey(ctx context.Context, persister operator.Persister, key string) ([]*reader.Metadata, error) {
	// Note: This function loads a specific archived checkpoint set (by key) as part of the ring buffer retention policy.
	// See the file-level comment above for details on retention and tradeoffs.
	encoded, err := persister.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if encoded == nil {
		return []*reader.Metadata{}, nil
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))

	// Decode the number of entries
	var knownFileCount int
	if err = dec.Decode(&knownFileCount); err != nil {
		return nil, fmt.Errorf("decoding file count: %w", err)
	}

	rmds := make([]*reader.Metadata, 0, knownFileCount)
	for i := 0; i < knownFileCount; i++ {
		rmd := new(reader.Metadata)
		if err = dec.Decode(rmd); err != nil {
			return nil, err
		}
		if rmd.FileAttributes == nil {
			rmd.FileAttributes = map[string]any{}
		}
		if ha, ok := rmd.FileAttributes["HeaderAttributes"]; ok {
			if hat, ok := ha.(map[string]any); ok {
				for k, v := range hat {
					rmd.FileAttributes[k] = v
				}
				delete(rmd.FileAttributes, "HeaderAttributes")
			}
		}
		rmds = append(rmds, rmd)
	}
	return rmds, nil
}

func LoadKey(ctx context.Context, persister operator.Persister, key string) ([]*reader.Metadata, error) {
	encoded, err := persister.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if encoded == nil {
		return []*reader.Metadata{}, nil
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))

	// Decode the number of entries
	var knownFileCount int
	if err = dec.Decode(&knownFileCount); err != nil {
		return nil, fmt.Errorf("decoding file count: %w", err)
	}

	rmds := make([]*reader.Metadata, 0, knownFileCount)
	for i := 0; i < knownFileCount; i++ {
		rmd := new(reader.Metadata)
		if err = dec.Decode(rmd); err != nil {
			return nil, err
		}
		if rmd.FileAttributes == nil {
			rmd.FileAttributes = map[string]any{}
		}
		
		if ha, ok := rmd.FileAttributes["HeaderAttributes"]; ok {
			if hat, ok := ha.(map[string]any); ok {
				for k, v := range hat {
					rmd.FileAttributes[k] = v
				}
				delete(rmd.FileAttributes, "HeaderAttributes")
			}
		}

		rmds = append(rmds, rmd)
	}

	return rmds, nil
}
