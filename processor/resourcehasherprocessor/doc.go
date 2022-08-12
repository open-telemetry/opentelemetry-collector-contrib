// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package resourcehasherprocessor implements a processor for removing overhead
// of trasmitting trhe full resource upstream to an endpoint, using hashing to
// avoid repetition in a certain timewindow.
package resourcehasherprocessor // import "github.com/lumigo-io/opentelemetry-collector-contrib/processor/resourcehasherprocessor"
