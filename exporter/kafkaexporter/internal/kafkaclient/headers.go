// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"

	"go.opentelemetry.io/collector/client"
)

// metadataToHeaders converts metadata from the context into a slice of headers using the provided header constructor.
// This function is generic and can be used for both Sarama and Franz-go header types.
func metadataToHeaders[H any](ctx context.Context, keys []string,
	makeHeader func(key string, value []byte) H,
) []H {
	if len(keys) == 0 {
		return nil
	}
	info := client.FromContext(ctx)
	headers := make([]H, 0, len(keys))
	for _, key := range keys {
		valueSlice := info.Metadata.Get(key)
		for _, v := range valueSlice {
			headers = append(headers, makeHeader(key, []byte(v)))
		}
	}
	return headers
}

// setMessageHeaders is a generic helper for setting headers on a slice of messages.
// - messages: the messages to set headers on
// - ctx: context for extracting metadata
// - metadataKeys: which metadata keys to extract
// - makeHeader: constructs the header type for the target client (Sarama/Franz-go)
// - getHeaders: gets the headers from a message
// - setHeaders: sets the headers on a message
// Usage example (Sarama):
//
//	setMessageHeaders(ctx, allMessages, keys, makeHeader, getHeaders, setHeaders)
func setMessageHeaders[M any, H any](ctx context.Context,
	messages []M,
	metadataKeys []string,
	makeHeader func(key string, value []byte) H,
	getHeaders func(M) []H,
	setHeadersFunc func(M, []H),
) {
	setHeaders(
		messages,
		metadataToHeaders(ctx, metadataKeys, makeHeader),
		getHeaders,
		setHeadersFunc,
	)
}

// setHeaders sets or appends headers on each message in messages using the provided get/set functions.
func setHeaders[M any, H any](messages []M, headers []H,
	getHeaders func(M) []H,
	setHeaders func(M, []H),
) {
	if len(headers) == 0 || len(messages) == 0 {
		return
	}
	for i := range messages {
		h := getHeaders(messages[i])
		if len(h) == 0 {
			setHeaders(messages[i], headers)
		} else {
			setHeaders(messages[i], append(h, headers...))
		}
	}
}
