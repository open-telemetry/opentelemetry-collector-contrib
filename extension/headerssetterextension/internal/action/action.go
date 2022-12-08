// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
