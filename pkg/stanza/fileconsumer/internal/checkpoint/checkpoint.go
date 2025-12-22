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

	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

var ProtobufEncodingFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"filelog.protobufCheckpointEncoding",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Use protobuf encoding for checkpoint storage instead of JSON. Provides ~7x faster decoding and 31% storage savings."),
	featuregate.WithRegisterFromVersion("v0.143.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43266"),
)

const knownFilesKey = "knownFiles"

// Save syncs the most recent set of files to the database
// Uses protobuf encoding if the feature gate is enabled, otherwise uses JSON
func Save(ctx context.Context, persister operator.Persister, rmds []*reader.Metadata) error {
	return SaveKey(ctx, persister, rmds, knownFilesKey)
}

func shouldUseProtobuf() bool {
	return ProtobufEncodingFeatureGate.IsEnabled()
}

func SaveKey(ctx context.Context, persister operator.Persister, rmds []*reader.Metadata, key string, ops ...*storage.Operation) error {
	// Use protobuf if feature gate is enabled
	if shouldUseProtobuf() {
		return SaveKeyProto(ctx, persister, rmds, key, ops...)
	}

	// Otherwise use JSON (default)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	// Encode the number of known files
	if err := enc.Encode(len(rmds)); err != nil {
		return fmt.Errorf("encode num files: %w", err)
	}

	var errs error
	// Encode each known file
	for _, rmd := range rmds {
		if err := enc.Encode(rmd); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("encode metadata: %w", err))
		}
	}
	ops = append(ops, storage.SetOperation(key, buf.Bytes()))
	if err := persister.Batch(ctx, ops...); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("persist known files: %w", err))
	}

	return errs
}

// Load loads the most recent set of files from the database
// Tries protobuf first for backward compatibility, falls back to JSON if protobuf fails
func Load(ctx context.Context, persister operator.Persister) ([]*reader.Metadata, error) {
	return LoadKey(ctx, persister, knownFilesKey)
}

func LoadKey(ctx context.Context, persister operator.Persister, key string) ([]*reader.Metadata, error) {
	encoded, err := persister.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if encoded == nil {
		return []*reader.Metadata{}, nil
	}

	// Try protobuf first (for backward compatibility with existing protobuf checkpoints)
	// This allows seamless migration even when the feature gate is disabled
	rmds, err := tryLoadProtobuf(encoded)
	if err == nil {
		return rmds, nil
	}

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
