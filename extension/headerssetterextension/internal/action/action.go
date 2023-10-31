// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package action // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/action"

import "net/http"

type Action interface {
	ApplyOnHeaders(http.Header, string)
	ApplyOnMetadata(map[string]string, string)
}

type Insert struct {
	Key string
}

func (a Insert) ApplyOnHeaders(header http.Header, value string) {
	if v := header.Get(a.Key); v == "" {
		header.Set(a.Key, value)
	}
}

func (a Insert) ApplyOnMetadata(metadata map[string]string, value string) {
	if _, ok := metadata[a.Key]; !ok {
		metadata[a.Key] = value
	}
}

type Update struct {
	Key string
}

func (a Update) ApplyOnHeaders(header http.Header, value string) {
	if v := header.Get(a.Key); v != "" {
		header.Set(a.Key, value)
	}
}

func (a Update) ApplyOnMetadata(metadata map[string]string, value string) {
	if _, ok := metadata[a.Key]; ok {
		metadata[a.Key] = value
	}
}

type Delete struct {
	Key string
}

func (a Delete) ApplyOnHeaders(header http.Header, _ string) {
	header.Del(a.Key)
}

func (a Delete) ApplyOnMetadata(metadata map[string]string, _ string) {
	delete(metadata, a.Key)
}

type Upsert struct {
	Key string
}

func (a Upsert) ApplyOnHeaders(header http.Header, value string) {
	header.Set(a.Key, value)
}

func (a Upsert) ApplyOnMetadata(metadata map[string]string, value string) {
	metadata[a.Key] = value
}
