// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import "context"

type closeImmediately struct{}

func newRoller() roller {
	return &closeImmediately{}
}

func (r *closeImmediately) readLostFiles(ctx context.Context, readers []*Reader) {
	return
}

func (r *closeImmediately) roll(_ context.Context, readers []*Reader) {
	for _, reader := range readers {
		reader.Close()
	}
}

func (r *closeImmediately) cleanup() {
	return
}
