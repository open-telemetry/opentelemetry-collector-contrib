// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// s3RoutedEncoding pairs an S3Encoding with its pre-split path pattern parts.
type s3RoutedEncoding struct {
	enc          S3Encoding
	patternParts []string
}

// s3LogsDecoderRouter routes S3 object keys to the appropriate LogsDecoderFactory
// based on path pattern matching with wildcards.
//
// Encodings must be pre-sorted by specificity (most specific first, catch-all last).
// Use S3Config.sortedEncodings() to obtain a correctly sorted slice.
type s3LogsDecoderRouter struct {
	encodings []s3RoutedEncoding
	decoders  map[string]encoding.LogsDecoderFactory
}

// newS3LogsDecoderRouter creates a new S3 logs decoder router.
//   - encodings: pre-sorted S3Encoding slice.
//   - decoders: maps encoding.Name => LogsDecoderFactory for every entry (including raw-passthrough ones).
//
// Path patterns are split on "/" at construction time so that GetDecoder only
// splits the object key once per S3 event rather than once per pattern.
func newS3LogsDecoderRouter(
	encodings []S3Encoding,
	decoders map[string]encoding.LogsDecoderFactory,
) *s3LogsDecoderRouter {
	routed := make([]s3RoutedEncoding, len(encodings))
	for i, enc := range encodings {
		routed[i] = s3RoutedEncoding{
			enc:          enc,
			patternParts: strings.Split(enc.resolvePathPattern(), "/"),
		}
	}
	return &s3LogsDecoderRouter{
		encodings: routed,
		decoders:  decoders,
	}
}

// GetDecoder returns the LogsDecoderFactory and encoding name for the given S3 object key.
// It splits the object key once, then iterates encodings in order (most specific first)
// and returns on the first match. Returns an error if no encoding matches.
func (r *s3LogsDecoderRouter) GetDecoder(objectKey string) (encoding.LogsDecoderFactory, string, error) {
	targetParts := strings.Split(objectKey, "/")
	for _, re := range r.encodings {
		if !matchPrefixWithWildcard(targetParts, re.patternParts) {
			continue
		}
		decoder, ok := r.decoders[re.enc.Name]
		if !ok {
			return nil, "", fmt.Errorf("no decoder registered for encoding %q", re.enc.Name)
		}
		return decoder, re.enc.Name, nil
	}
	return nil, "", fmt.Errorf("no encoding matches S3 object key: %s", objectKey)
}
