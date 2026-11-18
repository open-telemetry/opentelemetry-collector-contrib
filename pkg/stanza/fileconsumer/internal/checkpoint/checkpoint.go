// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const knownFilesKey = "knownFiles"

// Save syncs the most recent set of files to the database using protobuf encoding
func Save(ctx context.Context, persister operator.Persister, rmds []*reader.Metadata) error {
	return SaveKey(ctx, persister, rmds, knownFilesKey)
}

// Load loads the most recent set of files from the database
// Tries protobuf first, falls back to JSON for checkpoints written by older versions
func Load(ctx context.Context, persister operator.Persister, logger *zap.Logger) ([]*reader.Metadata, error) {
	return LoadKey(ctx, persister, knownFilesKey, logger)
}

func LoadKey(ctx context.Context, persister operator.Persister, key string, logger *zap.Logger) ([]*reader.Metadata, error) {
	encoded, err := persister.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if encoded == nil {
		return []*reader.Metadata{}, nil
	}

	// Try protobuf first; JSON checkpoints from older versions fall through below
	rmds, err := tryLoadProtobuf(encoded)
	if err == nil {
		return rmds, nil
	}
	logger.Debug("failed to load checkpoint as protobuf, falling back to JSON", zap.Error(err))

	// Fall back to JSON if protobuf fails
	dec := json.NewDecoder(bytes.NewReader(encoded))

	// Decode the number of entries
	var knownFileCount int
	if err = dec.Decode(&knownFileCount); err != nil {
		return nil, fmt.Errorf("decoding file count: %w", err)
	}

	// Decode each of the known files
	var errs error
	rmds = make([]*reader.Metadata, 0, knownFileCount)
	for i := 0; i < knownFileCount; i++ {
		rmd := new(reader.Metadata)
		err = dec.Decode(rmd)
		if err != nil {
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
				maps.Copy(rmd.FileAttributes, hat)
				delete(rmd.FileAttributes, "HeaderAttributes")
			default:
				errs = multierr.Append(errs, errors.New("migrate header attributes: unexpected format"))
			}
		}

		// This reader won't be used for anything other than metadata reference, so just wrap the metadata
		rmds = append(rmds, rmd)
	}

	return rmds, errs
}
