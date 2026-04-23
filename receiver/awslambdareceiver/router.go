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

// cwRoutedEncoding pairs a CWEncoding with its pre-split log_group and log_stream
// pattern parts. A nil slice indicates the corresponding pattern is unset.
type cwRoutedEncoding struct {
	enc            CWEncoding
	logGroupParts  []string
	logStreamParts []string
}

// cwLogsDecoderRouter routes CloudWatch log events to the appropriate
// LogsDecoderFactory based on log_group and log_stream pattern matching.
//
// Encodings must be pre-sorted by specificity (most specific first, catch-all last).
// Use CloudWatchConfig.sortedEncodings() to obtain a correctly sorted slice.
type cwLogsDecoderRouter struct {
	encodings []cwRoutedEncoding
	decoders  map[string]encoding.LogsDecoderFactory
}

// newCWLogsDecoderRouter creates a new CloudWatch logs decoder router.
//   - encodings: pre-sorted CWEncoding slice.
//   - decoders: maps encoding.Name => LogsDecoderFactory for every entry (including raw-passthrough ones).
//
// Patterns are split on "/" at construction time so that GetDecoder only
// splits the log group / log stream once per event rather than once per pattern.
func newCWLogsDecoderRouter(
	encodings []CWEncoding,
	decoders map[string]encoding.LogsDecoderFactory,
) *cwLogsDecoderRouter {
	routed := make([]cwRoutedEncoding, len(encodings))
	for i, enc := range encodings {
		re := cwRoutedEncoding{enc: enc}
		if enc.LogGroupPattern != "" {
			re.logGroupParts = strings.Split(enc.LogGroupPattern, "/")
		}
		if enc.LogStreamPattern != "" {
			re.logStreamParts = strings.Split(enc.LogStreamPattern, "/")
		}
		routed[i] = re
	}
	return &cwLogsDecoderRouter{
		encodings: routed,
		decoders:  decoders,
	}
}

// GetDecoder returns the LogsDecoderFactory and encoding name for the given
// CloudWatch log group and log stream. Within a single entry, log_group_pattern
// is checked before log_stream_pattern. Across entries, iteration order is
// preserved (entries must be pre-sorted by specificity).
// Returns an error if no encoding matches.
func (r *cwLogsDecoderRouter) GetDecoder(logGroup, logStream string) (encoding.LogsDecoderFactory, string, error) {
	var groupParts, streamParts []string
	for _, re := range r.encodings {
		if re.logGroupParts != nil {
			if groupParts == nil {
				groupParts = strings.Split(logGroup, "/")
			}
			if matchPrefixWithWildcard(groupParts, re.logGroupParts) {
				return r.resolve(re.enc.Name)
			}
		}
		if re.logStreamParts != nil {
			if streamParts == nil {
				streamParts = strings.Split(logStream, "/")
			}
			if matchPrefixWithWildcard(streamParts, re.logStreamParts) {
				return r.resolve(re.enc.Name)
			}
		}
	}
	return nil, "", fmt.Errorf("no encoding matches CloudWatch log_group=%q log_stream=%q", logGroup, logStream)
}

func (r *cwLogsDecoderRouter) resolve(name string) (encoding.LogsDecoderFactory, string, error) {
	decoder, ok := r.decoders[name]
	if !ok {
		return nil, "", fmt.Errorf("no decoder registered for encoding %q", name)
	}
	return decoder, name, nil
}
