// Copyright  The OpenTelemetry Authors
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

// Package race exists so that it can be possible to check if the
// race detector has been added into the build to allow for tests
// that provide no unit test value but perform concurrent actions
// where the race detector can identify issues.
package race // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/race"
