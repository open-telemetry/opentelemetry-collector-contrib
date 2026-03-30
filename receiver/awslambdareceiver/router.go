// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// logsDecoderRouter routes S3 object keys to the appropriate LogsDecoderFactory
// based on path pattern matching with wildcards.
//
// Encodings must be pre-sorted by specificity (most specific first, catch-all last).
// Use S3Config.SortedEncodings() to obtain a correctly sorted slice.
type logsDecoderRouter struct {
	encodings      []S3Encoding
	decoders       map[string]encoding.LogsDecoderFactory
	defaultDecoder encoding.LogsDecoderFactory
}

// newLogsDecoderRouter creates a new S3 logs decoder router.
//   - encodings: pre-sorted S3Encoding slice (use S3Config.SortedEncodings()).
//   - decoders: maps encoding.Name => LogsDecoderFactory for entries with an Encoding field set.
//   - defaultDecoder: used for entries with no Encoding (raw passthrough).
func newLogsDecoderRouter(
	encodings []S3Encoding,
	decoders map[string]encoding.LogsDecoderFactory,
	defaultDecoder encoding.LogsDecoderFactory,
) *logsDecoderRouter {
	return &logsDecoderRouter{
		encodings:      encodings,
		decoders:       decoders,
		defaultDecoder: defaultDecoder,
	}
}

// GetDecoder returns the LogsDecoderFactory and encoding name for the given S3 object key.
// It iterates encodings in order (most specific first) and returns on the first match.
// Returns an error if no encoding matches.
func (r *logsDecoderRouter) GetDecoder(objectKey string) (encoding.LogsDecoderFactory, string, error) {
	for _, enc := range r.encodings {
		pattern := enc.ResolvePathPattern()
		if !matchPrefixWithWildcard(objectKey, pattern) {
			continue
		}
		// No encoding => raw passthrough using the default decoder.
		if enc.Encoding == "" {
			return r.defaultDecoder, enc.Name, nil
		}
		decoder, ok := r.decoders[enc.Name]
		if !ok {
			return nil, "", fmt.Errorf("no decoder registered for encoding %q", enc.Name)
		}
		return decoder, enc.Name, nil
	}
	return nil, "", fmt.Errorf("no encoding matches S3 object key: %s", objectKey)
}
