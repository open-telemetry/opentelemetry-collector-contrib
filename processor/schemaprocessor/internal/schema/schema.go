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

package schema

// Schema abstracts the parsed translation file
// into a useable
type Schema interface {
	SupportedVersion(version *Version) bool
}

// NoopSchema is used when the schemaURL is not valid
// to help reduce the amount of branching that would be
// needed in the event that nil was returned in its place
type NoopSchema struct{}

var _ Schema = (*NoopSchema)(nil)

func (NoopSchema) SupportedVersion(_ *Version) bool { return false }
