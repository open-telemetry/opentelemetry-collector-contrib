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

		// Migrate readers that used FileAttributes.HeaderAttributes
		// This block can be removed in a future release, tentatively v0.90.0
		if ha, ok := rmd.FileAttributes["HeaderAttributes"]; ok {
			switch hat := ha.(type) {
			case map[string]any:
				for k, v := range hat {
					rmd.FileAttributes[k] = v
				}
				delete(rmd.FileAttributes, "HeaderAttributes")
			default:
				errs = append(errs, errors.New("migrate header attributes: unexpected format"))
			}
		}

		// This reader won't be used for anything other than metadata reference, so just wrap the metadata
		rmds = append(rmds, rmd)
	}

	return rmds, errors.Join(errs...)
}
